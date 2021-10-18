// Package reader is used for writing image records to the database.
package reader

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"go.uber.org/zap"

	"github.com/itsHabib/sim/internal/images"
)

const (
	loggerName = "images.reader"
)

// Service provides the implementation to read image records from a dynamodb
// table.
type Service struct {
	db     dynamodbiface.DynamoDBAPI
	logger *zap.Logger
	name   string
}

// NewService returns an instantiated instance of a service which has the
// following dependencies:
//
// logger: for structured logging
//
// db: for interacting with the dynamodb table
//
// name: the dynamodb table name
func NewService(logger *zap.Logger, db dynamodbiface.DynamoDBAPI, name string) (*Service, error) {
	s := Service{
		db:     db,
		logger: logger.Named(loggerName),
		name:   name,
	}

	if err := s.validate(); err != nil {
		return nil, err
	}

	s.logger.Info("successfully initialized image reader")

	return &s, nil
}

func (s *Service) validate() error {
	var missingDeps []string

	for _, tc := range []struct {
		dep string
		chk func() bool
	}{
		{
			dep: "db",
			chk: func() bool { return s.db != nil },
		},
		{
			dep: "logger",
			chk: func() bool { return s.logger != nil },
		},
		{
			dep: "db table name",
			chk: func() bool { return s.name != "" },
		},
	} {
		if !tc.chk() {
			missingDeps = append(missingDeps, tc.dep)
		}
	}

	if len(missingDeps) > 0 {
		return fmt.Errorf(
			"unable to initialize service due to (%d) missing dependencies: %s",
			len(missingDeps),
			strings.Join(missingDeps, ","),
		)
	}

	return nil
}

// Get returns an image record given the id. Returns ErrRecordNotFound if no
// image is found by that ID.
func (s *Service) Get(id string) (*images.Record, error) {
	logger := s.logger.With(zap.String("imageId", id))

	input := dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"id": {
				S: aws.String(id),
			},
		},
		TableName: &s.name,
	}

	res, err := s.db.GetItem(&input)
	if err != nil {
		const msg = "unable to get item"
		logger.Error(msg, zap.Error(err))
		return nil, fmt.Errorf(msg+": %w", err)
	}
	// TODO: double check if this is how it is actually done for dynamodb,
	//  according to github issues seems so.
	if len(res.Item) == 0 {
		logger.Error("record not found", zap.Error(err))
		return nil, images.ErrRecordNotFound
	}

	var rec images.Record
	if err := dynamodbattribute.UnmarshalMap(res.Item, &rec); err != nil {
		const msg = "unable to unmarshal map into record"
		logger.Error(msg, zap.Error(err))
		return nil, fmt.Errorf(msg+": %w", err)
	}

	return &rec, nil
}

// List lists all the image records in the db. This performs a scan
// operation which can be slow with many items in the db.
func (s *Service) List() ([]images.Record, error) {
	input := dynamodb.ScanInput{
		TableName: aws.String(s.name),
	}

	// get items
	resp, err := s.db.Scan(&input)
	if err != nil {
		const msg = "unable to scan database"
		s.logger.Error(msg, zap.Error(err))
		return nil, fmt.Errorf(msg+": %w", err)
	}

	// convert to records
	records := make([]images.Record, len(resp.Items))
	if err := dynamodbattribute.UnmarshalListOfMaps(resp.Items, &records); err != nil {
		const msg = "unable to unmarshal list"
		s.logger.Error(msg, zap.Error(err))
		return nil, fmt.Errorf(msg+": %w", err)
	}

	return records, nil
}

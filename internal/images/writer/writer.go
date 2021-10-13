// Package writer is used for writing image records to the database.
package writer

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
	loggerName = "images.writer"
)

// Service provides the implementation to write image records to a dynamodb
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

	s.logger.Info("successfully initialized image writer")

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

// Create adds the given record to the dynamodb table.
func (s *Service) Create(record *images.Record) error {
	logger := s.logger.With(
		zap.String("recordId", record.ID),
		zap.String("key", record.Key),
		zap.String("storage", record.Storage),
	)

	av, err := dynamodbattribute.MarshalMap(record)
	if err != nil {
		const msg = "unable to marshal image record"
		logger.Error(msg, zap.Error(err))
		return fmt.Errorf(msg+": %w", err)
	}

	input := dynamodb.PutItemInput{
		TableName: aws.String(s.name),
		Item:      av,
	}

	// attempt to put item
	if _, err := s.db.PutItem(&input); err != nil {
		const msg = "unable to put item in db"
		logger.Error(msg, zap.Error(err))
		return fmt.Errorf(msg+": %w", err)
	}

	logger.Info("successfully put item in db")

	return nil
}

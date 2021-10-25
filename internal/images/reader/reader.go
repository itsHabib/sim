// Package reader is used for writing image records to the database.
package reader

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/couchbase/gocb/v2"
	"go.uber.org/zap"

	"github.com/itsHabib/sim/internal/images"
)

const (
	loggerName = "images.reader"
	dbTimeout  = time.Second * 3
)

// Service provides the implementation to read image records from a dynamodb
// table.
type Service struct {
	cb         *gocb.Cluster
	collection *gocb.Collection
	logger     *zap.Logger
	name       string
}

// NewService returns an instantiated instance of a service which has the
// following dependencies:
//
// logger: for structured logging
//
// cb: couchbase cluster connection
//
// name: the couchbase bucket name
func NewService(logger *zap.Logger, cb *gocb.Cluster, name string) (*Service, error) {
	s := Service{
		cb:     cb,
		logger: logger.Named(loggerName),
		name:   name,
	}
	if err := s.setCollection(cb, name); err != nil {
		const msg = "unable to set collection"
		s.logger.Error(msg, zap.Error(err))
		return nil, fmt.Errorf(msg+": %w", err)
	}

	if err := s.validate(); err != nil {
		return nil, err
	}

	s.logger.Debug("successfully initialized image reader")

	return &s, nil
}

func (s *Service) validate() error {
	var missingDeps []string

	for _, tc := range []struct {
		dep string
		chk func() bool
	}{
		{
			dep: "cb",
			chk: func() bool { return s.cb != nil },
		},
		{
			dep: "collection",
			chk: func() bool { return s.collection != nil },
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

	options := gocb.GetOptions{
		Timeout: dbTimeout,
	}
	res, err := s.collection.Get(id, &options)
	if err != nil {
		if errors.Is(err, gocb.ErrDocumentNotFound) {
			logger.Error("record not found")
			return nil, images.ErrRecordNotFound
		}
		const msg = "unable to get image by id"
		logger.Error(msg, zap.Error(err))
		return nil, fmt.Errorf(msg+": %w", err)
	}

	var rec images.Record
	if err := res.Content(&rec); err != nil {
		const msg = "unable to unmarshal result into image record"
		logger.Error(msg, zap.Error(err))
		return nil, fmt.Errorf(msg+": %w", err)
	}

	return &rec, nil
}

// List lists all the image records in the db. This performs a scan
// operation which can be slow with many items in the db. Returns an ErrRecordNotFound
// if no records are found.
func (s *Service) List() ([]images.Record, error) {

	fqn := "`" + s.name + "`" + "." + images.Scope + "." + images.Collection
	query := "SELECT * FROM " + fqn

	options := gocb.QueryOptions{
		Timeout: dbTimeout,
	}
	result, err := s.cb.Query(query, &options)
	if err != nil {
		const msg = "unable to query cluster"
		s.logger.Error(msg, zap.Error(err))
		return nil, fmt.Errorf(msg+": %w", err)
	}

	var list []images.Record
	for result.Next() {
		var rec images.Record
		if err := result.Row(&rec); err != nil {
			const msg = "unable to unmarshal result into image record"
			s.logger.Error(msg, zap.Error(err))
			return nil, fmt.Errorf(msg+": %w", err)
		}
		list = append(list, rec)
	}

	if len(list) == 0 {
		return nil, images.ErrRecordNotFound
	}

	return list, nil
}

func (s *Service) setCollection(c *gocb.Cluster, bucket string) error {
	b := c.Bucket(bucket)
	if err := b.WaitUntilReady(time.Second*3, nil); err != nil {
		return fmt.Errorf("unable to connect to bucket: %q", err)
	}

	s.collection = b.Scope(images.Scope).Collection(images.Collection)

	return nil
}

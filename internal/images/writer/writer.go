// Package writer is used for writing image records to the database.
package writer

import (
	"fmt"
	"strings"
	"time"

	"github.com/couchbase/gocb/v2"
	"go.uber.org/zap"

	"github.com/itsHabib/sim/internal/images"
)

const (
	loggerName = "images.writer"
	dbTimeout  = time.Second * 3
)

// Service provides the implementation to write image records to a dynamodb
// table.
type Service struct {
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

	s.logger.Debug("successfully initialized image writer")

	return &s, nil
}

func (s *Service) validate() error {
	var missingDeps []string

	for _, tc := range []struct {
		dep string
		chk func() bool
	}{
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

// Create adds the given record to the dynamodb table.
func (s *Service) Create(record *images.Record) error {
	logger := s.logger.With(
		zap.String("recordId", record.ID),
		zap.String("key", record.Key),
		zap.String("storage", record.Storage),
	)

	// attempt to insert item
	options := gocb.InsertOptions{
		DurabilityLevel: gocb.DurabilityLevelNone,
		Timeout:         dbTimeout,
	}
	if _, err := s.collection.Insert(record.ID, record, &options); err != nil {
		const msg = "unable to insert image record"
		logger.Error(msg, zap.Error(err))
		return fmt.Errorf(msg+": %w", err)
	}

	logger.Info("successfully inserted item in db")

	return nil
}

// Delete removes the item with id from the database.
func (s *Service) Delete(id string) error {
	logger := s.logger.With(zap.String("imageId", id))

	if _, err := s.collection.Remove(id, &gocb.RemoveOptions{Timeout: dbTimeout}); err != nil {
		const msg = "unable to delete image record"
		logger.Error(msg, zap.Error(err))
		return fmt.Errorf(msg+": %w", err)
	}

	logger.Info("successfully deleted item from db")

	return nil
}

func (s *Service) setCollection(c *gocb.Cluster, bucket string) error {
	b := c.Bucket(bucket)
	if err := b.WaitUntilReady(time.Second*3, nil); err != nil {
		return fmt.Errorf("unable to connect to bucket: %q", err)
	}

	s.collection = b.Scope(images.Scope).Collection(images.Collection)

	return nil
}

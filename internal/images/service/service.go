package service

import (
	"fmt"
	"go.uber.org/zap"
	"strings"

	"github.com/itsHabib/sim/internal/images"
)

const (
	loggerName = "images.service"
	region     = "us-east-1"
)

// Service provides the implementation for interacting with images.
type Service struct {
	logger *zap.Logger
	reader images.Reader
	writer images.Writer
}

// NewService returns an instantiated instance of a service which has the
// following dependencies:
//
// logger: for structured logging
//
// reader: for reading image records
//
// writer: for writing image records
func NewService(logger *zap.Logger, reader images.Reader, writer images.Writer) (*Service, error) {
	s := Service{
		logger: logger.Named(loggerName),
		reader: reader,
		writer: writer,
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
			dep: "logger",
			chk: func() bool { return s.logger != nil },
		},
		{
			dep: "reader",
			chk: func() bool { return s.reader != nil },
		},
		{
			dep: "writer",
			chk: func() bool { return s.writer != nil },
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

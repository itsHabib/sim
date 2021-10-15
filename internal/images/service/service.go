package service

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/itsHabib/sim/internal/images"
)

const (
	loggerName = "images.service"
	region     = "us-east-1"
)

// Service provides the implementation for interacting with images.
type Service struct {
	logger  *zap.Logger
	storage string
	reader  images.Reader
	writer  images.Writer
}

// NewService returns an instantiated instance of a service which has the
// following dependencies:
//
// logger: for structured logging
//
// storage: the cloud storage name i.e. AWS bucket
//
// reader: for reading image records
//
// writer: for writing image records
func NewService(logger *zap.Logger, storage string, reader images.Reader, writer images.Writer) (*Service, error) {
	s := Service{
		logger:  logger.Named(loggerName),
		storage: storage,
		reader:  reader,
		writer:  writer,
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

// Upload attempts to upload using the given request and adds a corresponding
// image record in the DB.
func (s *Service) Upload(r UploadRequest) error {
	logger := s.logger.With(zap.String("name", r.Name))
	logger.Info("attempting to upload")

	// get session
	sess, err := session.NewSession(aws.NewConfig().WithRegion(region))
	if err != nil {
		const msg = "unable to get AWS session"
		logger.Error(msg, zap.Error(err))
		return fmt.Errorf(msg+": %w", err)
	}

	// upload image
	imageID := uuid.New().String()
	s3Uploader := s3manager.NewUploader(sess)
	s3Client := s3.New(sess)
	key := uploadKey(r, imageID)
	uploadInput := s3manager.UploadInput{
		ACL:    aws.String("private"),
		Body:   r.Body,
		Bucket: &s.storage,
		Key:    &key,
	}
	if _, err := s3Uploader.Upload(&uploadInput); err != nil {
		const msg = "unable to upload image"
		logger.Error(msg, zap.Error(err))
		return fmt.Errorf(msg+": %w", err)
	}

	// head object to get the content length
	headInput := s3.HeadObjectInput{
		Bucket: &s.storage,
		Key:    &key,
	}
	resp, err := s3Client.HeadObject(&headInput)
	if err != nil {
		const msg = "unable to head object"
		logger.Error(msg, zap.Error(err))
		return fmt.Errorf(msg+": %w", err)
	}

	// create image record to point to this object
	now := time.Now().UTC()
	image := images.Record{
		ID:        imageID,
		CreatedAt: &now,
		ETag:      unwrapStr(resp.ETag),
		Key:       key,
		Name:      r.Name,
		Size:      bytesToKB(unwrapInt(resp.ContentLength)),
		Storage:   s.storage,
	}
	if err := s.writer.Create(&image); err != nil {
		const msg = "unable to create image record"
		logger.Error(msg, zap.Error(err))
		return fmt.Errorf(msg+": %w", err)
	}

	return nil
}

func (s *Service) Download(r DownloadRequest) error {
	logger := s.logger.With(zap.String("imageId", r.ID), zap.String("filePath", r.FilePath))
	logger.Info("attempting to download object")
	return nil

	//TODO

	// get record  from id
	//rec, err := s.reader.Get()
	//switch err {
	//case nil:
	//case
	//}

	//
	//f, err := os.Open(r.FilePath)
	//if err != nil {
	//	const msg = "unable to open file"
	//	logger.Error(msg, zap.Error(err))
	//	return fmt.Errorf(msg+": %w", err)
	//}
	//
	//// get session
	//sess, err := session.NewSession(aws.NewConfig().WithRegion(region))
	//if err != nil {
	//	const msg = "unable to get AWS session"
	//	logger.Error(msg, zap.Error(err))
	//	return fmt.Errorf(msg+": %w", err)
	//}
	//s3Downloader := s3manager.NewDownloader(sess)
}

// DownloadRequest represents the type used to request a download on an
// io.Reader to cloud storage.
type DownloadRequest struct {
	// ID of the image.
	ID string

	// FilePath to download the object into
	FilePath string
}

// UploadRequest represents the type used to request an upload on an io.Reader
// to cloud storage.
type UploadRequest struct {
	// Name of the file to upload
	Name string

	// Body of the data to upload
	Body io.Reader
}

func uploadKey(r UploadRequest, imageID string) string {
	return "images/" + imageID + "/" + r.Name
}

func unwrapInt(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

func unwrapStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func bytesToKB(b int64) int64 {
	return b / 1024
}

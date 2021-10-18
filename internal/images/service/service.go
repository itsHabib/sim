package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
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
	logger        *zap.Logger
	reader        images.Reader
	sessionGetter images.SessionGetter
	storage       string
	writer        images.Writer
}

// New returns an instantiated instance of a service which has the
// following dependencies:
//
// logger: for structured logging
//
// storage: the cloud storage name i.e. AWS bucket
//
// reader: for reading image records
//
// writer: for writing image records
//
// sessionGetter: for configuring the AWS session
func New(logger *zap.Logger, storage string, reader images.Reader, writer images.Writer, sessionGetter images.SessionGetter) (*Service, error) {
	s := Service{
		logger:        logger.Named(loggerName),
		sessionGetter: sessionGetter,
		storage:       storage,
		reader:        reader,
		writer:        writer,
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

// Delete will remove both the image from cloud storage and the DB record
// that represents the image.
func (s *Service) Delete(id string) error {
	logger := s.logger.With(zap.String("imageId", id))

	//get record  from id
	rec, err := s.reader.Get(id)
	switch err {
	case nil:
	case images.ErrRecordNotFound:
		logger.Error("record not found", zap.Error(err))
		return err
	default:
		const msg = "unable to retrieve image record"
		logger.Error(msg, zap.Error(err))
		return fmt.Errorf(msg+": %w", err)
	}

	// delete image object
	if err := s.deleteObject(rec.Key, logger); err != nil {
		const msg = "unable to delete object"
		logger.Error(msg, zap.Error(err))
		return fmt.Errorf(msg+": %w", err)
	}

	// remove record from db
	err = s.writer.Delete(id)
	switch err {
	case nil, images.ErrRecordNotFound:
		return nil
	default:
		const msg = "unable to delete record"
		logger.Error(msg, zap.Error(err))
		return fmt.Errorf(msg+": %w", err)
	}
}

// Download attempts to download an image file from cloud storage to the
// requested file path.
func (s *Service) Download(r images.DownloadRequest) error {
	logger := s.logger.With(zap.String("imageId", r.ID))
	logger.Info("attempting to download object")

	//get record  from id
	rec, err := s.reader.Get(r.ID)
	switch err {
	case nil:
	case images.ErrRecordNotFound:
		logger.Error("record not found", zap.Error(err))
		return err
	default:
		const msg = "unable to retrieve image record"
		logger.Error(msg, zap.Error(err))
		return fmt.Errorf(msg+": %w", err)
	}

	// get downloader
	sess, err := s.sessionGetter()
	if err != nil {
		const msg = "unable to get AWS session"
		logger.Error(msg, zap.Error(err))
		return fmt.Errorf(msg+": %w", err)
	}
	s3Downloader := s3manager.NewDownloader(sess)

	// download
	input := s3.GetObjectInput{
		Bucket: &s.storage,
		Key:    &rec.Key,
	}
	if _, err := s3Downloader.Download(r.Stream, &input); err != nil {
		const msg = "unable to download file"
		logger.Error(msg, zap.Error(err))
		if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == s3.ErrCodeNoSuchKey {
			return images.ErrObjectNotFound
		}
		return fmt.Errorf(msg+": %w", err)
	}
	logger.Info("successfully downloaded file")

	return nil
}

// Upload attempts to upload using the given request and adds a corresponding
// image record in the DB.
func (s *Service) Upload(r images.UploadRequest) (string, error) {
	logger := s.logger.With(zap.String("name", r.Name))
	logger.Info("attempting to upload")

	// get session
	sess, err := s.sessionGetter()
	if err != nil {
		const msg = "unable to get AWS session"
		logger.Error(msg, zap.Error(err))
		return "", fmt.Errorf(msg+": %w", err)
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
		return "", fmt.Errorf(msg+": %w", err)
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
		return "", fmt.Errorf(msg+": %w", err)
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
		return "", fmt.Errorf(msg+": %w", err)
	}
	logger.Info("successfully uploaded file")

	return imageID, nil
}

func (s *Service) deleteObject(key string, logger *zap.Logger) error {
	sess, err := s.sessionGetter()
	if err != nil {
		const msg = "unable to get AWS session"
		logger.Error(msg, zap.Error(err))
		return fmt.Errorf(msg+": %w", err)
	}
	client := s3.New(sess)

	input := s3.DeleteObjectInput{
		Bucket: &s.storage,
		Key:    &key,
	}
	if _, err := client.DeleteObject(&input); err != nil {
		if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() != s3.ErrCodeNoSuchKey && strings.Contains(awsErr.Code(), "NotFound") {
			logger.Info("object not found")
			return nil
		}
		const msg = "unable to delete object"
		logger.Error(msg, zap.Error(err))
		return fmt.Errorf(msg+": %w", err)
	}

	return nil
}

func uploadKey(r images.UploadRequest, imageID string) string {
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

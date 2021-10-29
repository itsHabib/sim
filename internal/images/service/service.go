package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/itsHabib/sim/internal/images"
	internalS3 "github.com/itsHabib/sim/internal/s3"
)

const (
	loggerName = "images.service"
	region     = "us-east-1"
)

// Service provides the implementation for interacting with images.
type Service struct {
	logger        *zap.Logger
	reader        images.Reader
	sdk           *sdk
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
		sdk:           new(sdk),
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
	s.sdk.init(withSDKDownloader(sess))

	// download
	input := s3.GetObjectInput{
		Bucket: &s.storage,
		Key:    &rec.Key,
	}
	if _, err := s.sdk.downloader.Download(r.Stream, &input); err != nil {
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

// List returns a list all the image records stored in the database.
func (s *Service) List() ([]images.Image, error) {
	records, err := s.reader.List()
	switch err {
	case nil:
	case images.ErrRecordNotFound:
		return nil, err
	default:
		const msg = "unable to list records"
		s.logger.Error(msg, zap.Error(err))
		return nil, fmt.Errorf(msg+": %w", err)
	}

	resp := make([]images.Image, len(records))
	for i := range records {
		resp[i] = images.Image{
			ID:          records[i].ID,
			Name:        records[i].Name,
			SizeInBytes: records[i].SizeInBytes,
		}
	}

	return resp, nil
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
	s.sdk.init(withSDKClient(sess), withSDKUploader(sess))

	// upload image
	imageID := uuid.New().String()
	key := uploadKey(r, imageID)
	uploadInput := s3manager.UploadInput{
		ACL:    aws.String("private"),
		Body:   r.Body,
		Bucket: &s.storage,
		Key:    &key,
	}
	if _, err := s.sdk.uploader.Upload(&uploadInput); err != nil {
		const msg = "unable to upload image"
		logger.Error(msg, zap.Error(err))
		return "", fmt.Errorf(msg+": %w", err)
	}

	// head object to get the content length
	headInput := s3.HeadObjectInput{
		Bucket: &s.storage,
		Key:    &key,
	}
	resp, err := s.sdk.client.HeadObject(&headInput)
	if err != nil {
		const msg = "unable to head object"
		logger.Error(msg, zap.Error(err))
		return "", fmt.Errorf(msg+": %w", err)
	}

	if resp.ETag == nil || resp.ContentLength == nil {
		const msg = "etag and/or content length is nil, unable to save metadata"
		logger.Error(msg)
		return "", errors.New(msg)
	}

	// create image record to point to this object
	now := time.Now().UTC()
	image := images.Record{
		ID:          imageID,
		CreatedAt:   &now,
		ETag:        *resp.ETag,
		Key:         key,
		Name:        r.Name,
		SizeInBytes: *resp.ContentLength,
		Storage:     s.storage,
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
	s.sdk.init(withSDKClient(sess))

	input := s3.DeleteObjectInput{
		Bucket: &s.storage,
		Key:    &key,
	}
	if _, err := s.sdk.client.DeleteObject(&input); err != nil {
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

type sdk struct {
	client     internalS3.Client
	downloader internalS3.Downloader
	uploader   internalS3.Uploader
}

func (s *sdk) init(opts ...sdkOpts) {
	for i := range opts {
		opts[i](s)
	}
}

type sdkOpts func(s *sdk)

func withSDKClient(sess *session.Session) sdkOpts {
	return func(s *sdk) {
		if s.client == nil {
			s.client = s3.New(sess)
		}
	}
}

func withSDKDownloader(sess *session.Session) sdkOpts {
	return func(s *sdk) {
		if s.downloader == nil {
			s.downloader = s3manager.NewDownloader(sess)
		}
	}
}

func withSDKUploader(sess *session.Session) sdkOpts {
	return func(s *sdk) {
		if s.uploader == nil {
			s.uploader = s3manager.NewUploader(sess)
		}
	}
}

func uploadKey(r images.UploadRequest, imageID string) string {
	return "images/" + imageID + "/" + r.Name
}

func bytesToKB(b int64) int64 {
	return b / 1024
}

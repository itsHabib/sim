package service

import (
	"errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/itsHabib/sim/internal/images"
	mock_images "github.com/itsHabib/sim/internal/images/mocks"
	internalS3 "github.com/itsHabib/sim/internal/s3"
	mock_s3 "github.com/itsHabib/sim/internal/s3/mocks"
)

func Test_Service_Upload(t *testing.T) {
	storage := "sim"
	r := images.UploadRequest{
		Name: "test",
		Body: strings.NewReader("hw"),
	}
	defaultMockUpload := func(ctrl *gomock.Controller, t *testing.T) internalS3.Uploader {
		u := mock_s3.NewMockUploader(ctrl)
		u.
			EXPECT().
			Upload(gomock.Any()).
			DoAndReturn(func(input *s3manager.UploadInput, options ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error) {
				require.NotNil(t, input.Key)
				assert.NotNil(t, input.Bucket)
				assert.Equal(t, storage, *input.Bucket)
				assert.Contains(t, *input.Key, "images/")
				assert.Contains(t, *input.Key, "test")
				assert.Equal(t, r.Body, input.Body)

				return new(s3manager.UploadOutput), nil
			})

		return u
	}
	defaultMockClient := func(ctrl *gomock.Controller) internalS3.Client {
		c := mock_s3.NewMockClient(ctrl)
		c.
			EXPECT().
			HeadObject(gomock.Any()).
			DoAndReturn(func(input *s3.HeadObjectInput) (*s3.HeadObjectOutput, error) {
				require.NotNil(t, input.Key)
				require.NotNil(t, input.Bucket)
				assert.Equal(t, storage, *input.Bucket)
				assert.Contains(t, *input.Key, "images/")
				assert.Contains(t, *input.Key, "test")

				return &s3.HeadObjectOutput{
					ContentLength: aws.Int64(1024),
					ETag:          aws.String("etag"),
				}, nil
			})

		return c
	}

	for _, tc := range []struct {
		desc          string
		client        func(ctrl *gomock.Controller) internalS3.Client
		uploader      func(ctrl *gomock.Controller, t *testing.T) internalS3.Uploader
		writer        func(ctrl *gomock.Controller) images.Writer
		sessionGetter images.SessionGetter
		wantErr       bool
	}{
		{
			desc:          "Upload() should return an error when failing to get the session",
			sessionGetter: func() (*session.Session, error) { return nil, errors.New("random") },
			wantErr:       true,
		},
		{
			desc:          "Upload() should return an error when failing to upload",
			sessionGetter: func() (*session.Session, error) { return new(session.Session), nil },
			uploader: func(ctrl *gomock.Controller, t *testing.T) internalS3.Uploader {
				u := mock_s3.NewMockUploader(ctrl)
				u.
					EXPECT().
					Upload(gomock.Any()).
					DoAndReturn(func(input *s3manager.UploadInput, options ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error) {
						require.NotNil(t, input.Key)
						require.NotNil(t, input.Bucket)
						assert.Equal(t, storage, *input.Bucket)
						assert.Contains(t, *input.Key, "images/")
						assert.Contains(t, *input.Key, "test")
						assert.Equal(t, r.Body, input.Body)

						return nil, errors.New("random")
					})

				return u
			},
			wantErr: true,
		},
		{
			desc:          "Upload() should return an error when failing to head object",
			sessionGetter: func() (*session.Session, error) { return new(session.Session), nil },
			uploader:      defaultMockUpload,
			client: func(ctrl *gomock.Controller) internalS3.Client {
				c := mock_s3.NewMockClient(ctrl)
				c.
					EXPECT().
					HeadObject(gomock.Any()).
					DoAndReturn(func(input *s3.HeadObjectInput) (*s3.HeadObjectOutput, error) {
						require.NotNil(t, input.Key)
						require.NotNil(t, input.Bucket)
						assert.Equal(t, storage, *input.Bucket)
						assert.Contains(t, *input.Key, "images/")
						assert.Contains(t, *input.Key, "test")

						return nil, errors.New("random")
					})

				return c
			},
			wantErr: true,
		},
		{
			desc:          "Upload() should return an error when the image writer fails",
			sessionGetter: func() (*session.Session, error) { return new(session.Session), nil },
			uploader:      defaultMockUpload,
			client:        defaultMockClient,
			writer: func(ctrl *gomock.Controller) images.Writer {
				w := mock_images.NewMockWriter(ctrl)
				w.
					EXPECT().
					Create(gomock.Any()).
					Return(errors.New("random"))

				return w
			},
			wantErr: true,
		},
		{
			desc:          "Upload() - happy path",
			sessionGetter: func() (*session.Session, error) { return new(session.Session), nil },
			uploader:      defaultMockUpload,
			client:        defaultMockClient,
			writer: func(ctrl *gomock.Controller) images.Writer {
				w := mock_images.NewMockWriter(ctrl)
				w.
					EXPECT().
					Create(gomock.Any()).
					DoAndReturn(func(i *images.Record) error {
						require.NotNil(t, i)
						assert.NotEmpty(t, i.CreatedAt)
						assert.Equal(t, "etag", i.ETag)
						assert.Equal(t, int64(1), i.Size)
						assert.Equal(t, "test", i.Name)
						assert.Equal(t, storage, i.Storage)

						return nil
					})

				return w
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			w := mock_images.NewMockWriter(ctrl)
			if tc.writer == nil {
				tc.writer = func(ctrl *gomock.Controller) images.Writer { return w }
			}
			u := mock_s3.NewMockUploader(ctrl)
			if tc.uploader == nil {
				tc.uploader = func(ctrl *gomock.Controller, _ *testing.T) internalS3.Uploader { return u }
			}
			c := mock_s3.NewMockClient(ctrl)
			if tc.client == nil {
				tc.client = func(ctrl *gomock.Controller) internalS3.Client { return c }
			}

			svc, err := New(zap.NewNop(), storage, mock_images.NewMockReader(ctrl), tc.writer(ctrl), tc.sessionGetter)
			svc.sdk.uploader = tc.uploader(ctrl, t)
			svc.sdk.client = tc.client(ctrl)
			require.NoError(t, err)

			s, err := svc.Upload(r)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, s)
			}
		})
	}
}

func mockSessionGetter() (*session.Session, error) {
	return new(session.Session), nil
}

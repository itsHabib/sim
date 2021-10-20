package service

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
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

func Test_Service_Delete(t *testing.T) {
	id := "id"
	storage := "storage"
	for _, tc := range []struct {
		desc    string
		reader  func(ctrl *gomock.Controller) images.Reader
		writer  func(ctrl *gomock.Controller) images.Writer
		client  func(ctrl *gomock.Controller) internalS3.Client
		wantErr bool
	}{
		{
			desc: "Delete() should return an error when failing to retrieve the object.",
			reader: func(ctrl *gomock.Controller) images.Reader {
				r := mock_images.NewMockReader(ctrl)
				r.
					EXPECT().
					Get(id).
					Return(nil, errors.New("random"))

				return r
			},
			writer:  func(ctrl *gomock.Controller) images.Writer { return mock_images.NewMockWriter(ctrl) },
			client:  func(ctrl *gomock.Controller) internalS3.Client { return mock_s3.NewMockClient(ctrl) },
			wantErr: true,
		},
		{
			desc: "Delete() should return an error when failing to delete the object in cloud storage.",
			reader: func(ctrl *gomock.Controller) images.Reader {
				r := mock_images.NewMockReader(ctrl)
				r.
					EXPECT().
					Get(id).
					Return(&images.Record{Key: "key"}, nil)

				return r
			},
			writer: func(ctrl *gomock.Controller) images.Writer { return mock_images.NewMockWriter(ctrl) },
			client: func(ctrl *gomock.Controller) internalS3.Client {
				c := mock_s3.NewMockClient(ctrl)
				c.
					EXPECT().
					DeleteObject(gomock.Any()).
					Return(nil, errors.New("random"))

				return c
			},
			wantErr: true,
		},
		{
			desc: "Delete() should return an error when failing to delete the object in the DB.",
			reader: func(ctrl *gomock.Controller) images.Reader {
				r := mock_images.NewMockReader(ctrl)
				r.
					EXPECT().
					Get(id).
					Return(&images.Record{Key: "key"}, nil)

				return r
			},
			writer: func(ctrl *gomock.Controller) images.Writer {
				w := mock_images.NewMockWriter(ctrl)
				w.
					EXPECT().
					Delete(id).
					Return(errors.New("random"))

				return w
			},
			client: func(ctrl *gomock.Controller) internalS3.Client {
				c := mock_s3.NewMockClient(ctrl)
				c.
					EXPECT().
					DeleteObject(gomock.Any()).
					Return(nil, nil)

				return c
			},
			wantErr: true,
		},
		{
			desc: "Delete() - happy path",
			reader: func(ctrl *gomock.Controller) images.Reader {
				r := mock_images.NewMockReader(ctrl)
				r.
					EXPECT().
					Get(id).
					Return(&images.Record{Key: "key"}, nil)

				return r
			},
			writer: func(ctrl *gomock.Controller) images.Writer {
				w := mock_images.NewMockWriter(ctrl)
				w.
					EXPECT().
					Delete(id).
					Return(nil)

				return w
			},
			client: func(ctrl *gomock.Controller) internalS3.Client {
				c := mock_s3.NewMockClient(ctrl)
				c.
					EXPECT().
					DeleteObject(gomock.Any()).
					Return(nil, nil)

				return c
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			svc, err := New(zap.NewNop(), storage, tc.reader(ctrl), tc.writer(ctrl), mockSessionGetter)
			svc.sdk.client = tc.client(ctrl)
			require.NoError(t, err)

			err = svc.Delete(id)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_Service_Download(t *testing.T) {
	id := "id"
	storage := "storage"
	req := images.DownloadRequest{
		ID: "id",
	}
	for _, tc := range []struct {
		desc       string
		reader     func(ctrl *gomock.Controller) images.Reader
		downloader func(t *testing.T, ctrl *gomock.Controller) internalS3.Downloader
		wantErr    bool
	}{
		{
			desc: "Download() should return an error when failing to retrieve the image record.",
			reader: func(ctrl *gomock.Controller) images.Reader {
				r := mock_images.NewMockReader(ctrl)
				r.
					EXPECT().
					Get(id).
					Return(nil, errors.New("random"))

				return r
			},
			wantErr: true,
		},
		{
			desc: "Download() should return an error when failing to download the object.",
			reader: func(ctrl *gomock.Controller) images.Reader {
				r := mock_images.NewMockReader(ctrl)
				r.
					EXPECT().
					Get(id).
					Return(&images.Record{Key: "key"}, nil)

				return r
			},
			downloader: func(t *testing.T, ctrl *gomock.Controller) internalS3.Downloader {
				c := mock_s3.NewMockDownloader(ctrl)
				c.
					EXPECT().
					Download(gomock.Any(), gomock.Any()).
					Return(int64(0), errors.New("random"))

				return c
			},
			wantErr: true,
		},
		{
			desc: "Download() - happy path",
			reader: func(ctrl *gomock.Controller) images.Reader {
				r := mock_images.NewMockReader(ctrl)
				r.
					EXPECT().
					Get(id).
					Return(&images.Record{Key: "key"}, nil)

				return r
			},
			downloader: func(t *testing.T, ctrl *gomock.Controller) internalS3.Downloader {
				c := mock_s3.NewMockDownloader(ctrl)
				c.
					EXPECT().
					Download(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ io.WriterAt, i *s3.GetObjectInput, _ ...func(*s3manager.Downloader)) (int64, error) {
						require.NotNil(t, i)
						assert.Equal(t, "key", unwrapStr(i.Key))
						assert.Equal(t, storage, unwrapStr(i.Bucket))

						return 10, nil
					})

				return c
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			r := mock_images.NewMockReader(ctrl)
			if tc.reader == nil {
				tc.reader = func(ctrl *gomock.Controller) images.Reader { return r }
			}
			d := mock_s3.NewMockDownloader(ctrl)
			if tc.downloader == nil {
				tc.downloader = func(_ *testing.T, ctrl *gomock.Controller) internalS3.Downloader { return d }
			}

			svc, err := New(zap.NewNop(), storage, tc.reader(ctrl), mock_images.NewMockWriter(ctrl), mockSessionGetter)
			svc.sdk.downloader = tc.downloader(t, ctrl)
			require.NoError(t, err)

			err = svc.Download(req)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

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

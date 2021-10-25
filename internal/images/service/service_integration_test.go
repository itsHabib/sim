//go:build integration
// +build integration

package service

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/couchbase/gocb/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/itsHabib/sim/internal/images"
	"github.com/itsHabib/sim/internal/images/reader"
	"github.com/itsHabib/sim/internal/images/writer"
)

var (
	localstack   string
	imageStorage string
	cbEndpoint   string
	cbUsername   string
	cbPassword   string
	cbBucket     string
)

func TestMain(m *testing.M) {
	var missingDeps []string
	for _, tc := range []struct {
		env string
		set func() bool
	}{
		{
			env: "LOCALSTACK_URL",
			set: func() bool {
				localstack = os.Getenv("LOCALSTACK_URL")
				return localstack != ""
			},
		},
		{
			env: "COUCHBASE_USERNAME",
			set: func() bool {
				cbUsername = os.Getenv("COUCHBASE_USERNAME")
				return cbUsername != ""
			},
		},
		{
			env: "COUCHBASE_PASSWORD",
			set: func() bool {
				cbPassword = os.Getenv("COUCHBASE_PASSWORD")
				return cbPassword != ""
			},
		},
		{
			env: "COUCHBASE_BUCKET",
			set: func() bool {
				cbBucket = os.Getenv("COUCHBASE_BUCKET")
				return cbBucket != ""
			},
		},
		{
			env: "COUCHBASE_ENDPOINT",
			set: func() bool {
				cbEndpoint = os.Getenv("COUCHBASE_ENDPOINT")
				return cbEndpoint != ""
			},
		},
		{
			env: "IMAGE_STORAGE",
			set: func() bool {
				imageStorage = os.Getenv("IMAGE_STORAGE")
				return imageStorage != ""
			},
		},
	} {
		if !tc.set() {
			missingDeps = append(missingDeps, tc.env)
		}
	}

	if len(missingDeps) > 0 {
		fmt.Printf("missing (%d) dependencies: %s\n", len(missingDeps), strings.Join(missingDeps, ", "))
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func Test_Service(t *testing.T) {
	var id string
	body := []byte("Hello, World!")
	for _, tc := range []struct {
		desc string
		do   func(svc *Service, t *testing.T)
		chk  func(svc *Service, t *testing.T)
	}{
		{
			desc: "Upload() should create the record and upload the object to cloud storage",
			do: func(svc *Service, t *testing.T) {
				r := images.UploadRequest{
					Name: "test",
					Body: bytes.NewReader(body),
				}
				var err error
				id, err = svc.Upload(r)
				require.Nil(t, err)
				require.NotEmpty(t, id)
			},
			chk: func(svc *Service, t *testing.T) {
				rec, err := svc.reader.Get(id)
				require.NoError(t, err)
				assert.Equal(t, id, rec.ID)
				assert.Equal(t, "test", rec.Name)
				assert.Equal(t, imageStorage, rec.Storage)
				assert.NotEmpty(t, rec.Key)
			},
		},
		{
			desc: "Download() should successfully download to the writer stream",
			do:   func(svc *Service, t *testing.T) {},
			chk: func(svc *Service, t *testing.T) {
				buffer := aws.NewWriteAtBuffer([]byte{})
				r := images.DownloadRequest{
					ID:     id,
					Stream: buffer,
				}
				require.NoError(t, svc.Download(r))
				assert.Equal(t, body, buffer.Bytes())
			},
		},
		{
			desc: "Delete() should remove both the object and record.",
			do: func(svc *Service, t *testing.T) {
				require.NoError(t, svc.Delete(id))
			},
			chk: func(svc *Service, t *testing.T) {
				r := images.UploadRequest{
					Name: "test",
				}
				sess := getSession(t)
				c := s3.New(sess)
				s3Input := s3.HeadObjectInput{
					Bucket: aws.String("sim"),
					Key:    aws.String(uploadKey(r, id)),
				}
				_, err := c.HeadObject(&s3Input)
				if err == nil {
					t.Fatal("expected object to be deleted")
				}
				if err != nil {
					if awsErr, ok := err.(awserr.Error); !ok || (awsErr.Code() != s3.ErrCodeNoSuchKey && !strings.Contains(awsErr.Code(), "NotFound")) {
						t.Fatalf("unexpected error while getting object: %v", awsErr)
					}
				}

				_, err = svc.reader.Get(id)
				assert.EqualError(t, err, images.ErrRecordNotFound.Error())
			},
		},
	} {
		if !t.Run(tc.desc, func(t *testing.T) {
			svc := getService(t)
			tc.do(svc, t)
			tc.chk(svc, t)
		}) {
			t.Fatalf("test ('%s') failed", tc.desc)
		}
	}
}

func getService(t *testing.T) *Service {
	nop := zap.NewNop()

	cb, err := getCluster()
	require.NoError(t, err)

	r, err := reader.NewService(nop, cb, cbBucket)
	require.NoError(t, err)

	w, err := writer.NewService(nop, cb, cbBucket)
	require.NoError(t, err)

	svc, err := New(zap.NewNop(), imageStorage, r, w, images.WithSessionOptions(getCfg()))
	require.NoError(t, err)

	return svc
}

func getCfg() *aws.Config {
	return aws.
		NewConfig().
		WithEndpoint(localstack).
		WithRegion(region).
		WithS3ForcePathStyle(true).
		WithCredentials(credentials.NewStaticCredentials("images", "secret", ""))
}

func getCluster() (*gocb.Cluster, error) {
	return gocb.Connect(
		cbEndpoint,
		gocb.ClusterOptions{
			Username: cbUsername,
			Password: cbPassword,
		},
	)
}

func getSession(t *testing.T) *session.Session {
	sess, err := session.NewSession(getCfg())
	require.NoError(t, err)

	return sess
}

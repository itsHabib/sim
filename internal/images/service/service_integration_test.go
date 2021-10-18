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
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/itsHabib/sim/internal/images"
	"github.com/itsHabib/sim/internal/images/reader"
	"github.com/itsHabib/sim/internal/images/writer"
)

var (
	localstack   string
	tableName    string
	imageStorage string
)

func TestMain(m *testing.M) {
	localstack = os.Getenv("LOCALSTACK_URL")
	if localstack == "" {
		fmt.Printf("LOCALSTACK_URL env var must be set\n")
		os.Exit(1)
	}

	tableName = os.Getenv("TABLE_NAME")
	if tableName == "" {
		fmt.Printf("TABLE_NAME env var must be set\n")
		os.Exit(1)
	}

	imageStorage = os.Getenv("IMAGE_STORAGE")
	if imageStorage == "" {
		fmt.Printf("IMAGE_STORAGE env var must be set\n")
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
			chk: func(svc *Service, t *testing.T) {},
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

				d := dynamodb.New(sess)
				dbInput := dynamodb.GetItemInput{
					Key: map[string]*dynamodb.AttributeValue{
						"id": {
							S: &id,
						},
					},
					TableName: aws.String("sim"),
				}
				resp, err := d.GetItem(&dbInput)
				require.NoError(t, err)
				require.Empty(t, resp.Item)
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
	sess := getSession(t)
	client := dynamodb.New(sess)

	r, err := reader.NewService(nop, client, tableName)
	require.NoError(t, err)

	w, err := writer.NewService(nop, client, tableName)
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

func getSession(t *testing.T) *session.Session {
	sess, err := session.NewSession(getCfg())
	require.NoError(t, err)

	return sess
}

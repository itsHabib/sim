//go:build integration
// +build integration

package service

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
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
	} {
		if !t.Run(tc.desc, func(t *testing.T) {
			svc := getService(t)
			tc.do(svc, t)
			tc.chk(svc, t)
		}) {
			return
		}
	}
}

func getService(t *testing.T) *Service {
	nop := zap.NewNop()
	cfg := aws.
		NewConfig().
		WithEndpoint(localstack).
		WithRegion(region).
		WithS3ForcePathStyle(true).
		WithCredentials(credentials.NewStaticCredentials("images", "secret", ""))
	sess, err := session.NewSession(cfg)
	require.NoError(t, err)

	client := dynamodb.New(sess)

	r, err := reader.NewService(nop, client, tableName)
	require.NoError(t, err)

	w, err := writer.NewService(nop, client, tableName)
	require.NoError(t, err)

	svc, err := New(zap.NewNop(), imageStorage, r, w, images.WithSessionOptions(cfg))
	require.NoError(t, err)

	return svc
}

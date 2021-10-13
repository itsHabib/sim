//go:build integration
// +build integration

package service

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/itsHabib/sim/internal/images"
	"github.com/itsHabib/sim/internal/images/reader"
	"github.com/itsHabib/sim/internal/images/writer"
)

var (
	localstack string
	tableName  string
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

	os.Exit(m.Run())
}

// TODO: once the service has methods we can use those instead of testing
//  the reader/writer directly
func Test_Service_DB(t *testing.T) {
	now := time.Now().UTC()
	record := images.Record{
		ID:        uuid.New().String(),
		CreatedAt: &now,
		Key:       "key",
		Storage:   "storage",
	}

	for _, tc := range []struct {
		desc string
		do   func(svc *Service, t *testing.T)
		chk  func(svc *Service, t *testing.T)
	}{
		{
			desc: "Create() should create the record with no errors",
			do: func(svc *Service, t *testing.T) {
				require.NoError(t, svc.writer.Create(&record))
			},
			chk: func(svc *Service, t *testing.T) {},
		},
		{
			desc: "List() should list the expected records",
			do:   func(svc *Service, t *testing.T) {},
			chk: func(svc *Service, t *testing.T) {
				records, err := svc.reader.List()
				require.NoError(t, err)

				require.NotEmpty(t, records)
				// find record by id instead of assuming there's only one item
				// we do this to avoid test failures when rerunning
				var found bool
				for i := range records {
					if records[i].ID == record.ID {
						assert.Equal(t, record, records[i])
						found = true
						break
					}
				}
				assert.True(t, found, "unable to find record in table")
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			svc := getService(t)
			tc.do(svc, t)
			tc.chk(svc, t)
		})
	}
}

func getService(t *testing.T) *Service {
	nop := zap.NewNop()
	cfg := aws.
		NewConfig().
		WithEndpoint(localstack).
		WithRegion(region).
		WithCredentials(credentials.NewStaticCredentials("images", "secret", ""))
	sess, err := session.NewSession(cfg)
	require.NoError(t, err)

	client := dynamodb.New(sess)

	r, err := reader.NewService(nop, client, tableName)
	require.NoError(t, err)

	w, err := writer.NewService(nop, client, tableName)
	require.NoError(t, err)

	svc, err := NewService(zap.NewNop(), r, w)
	require.NoError(t, err)

	return svc
}

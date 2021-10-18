package s3

import (
	"io"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

//go:generate go run github.com/golang/mock/mockgen -destination mocks/client.go github.com/itsHabib/sim/internal/s3 Client
//go:generate go run github.com/golang/mock/mockgen -destination mocks/downloader.go github.com/itsHabib/sim/internal/s3 Downloader
//go:generate go run github.com/golang/mock/mockgen -destination mocks/uploader.go github.com/itsHabib/sim/internal/s3 Uploader

// Client provides an abstraction to aid in mocking for unit tests
type Client interface {
	// HeadObject retrieves metadata from an object without returning the object
	// itself. This action is useful if you're only interested in an object's
	// metadata.
	HeadObject(input *s3.HeadObjectInput) (*s3.HeadObjectOutput, error)

	// DeleteObject removes the null version (if there is one) of an object and
	// inserts a delete marker, which becomes the latest version of the object.
	// If there isn't a null version, Amazon S3 does not remove any objects but
	// will still respond that the command was successful.
	DeleteObject(input *s3.DeleteObjectInput) (*s3.DeleteObjectOutput, error)
}

// Uploader provides an abstraction to aid in mocking for unit tests
type Uploader interface {
	// Upload uploads an object to S3, intelligently buffering large files into
	// smaller chunks and sending them in parallel across multiple goroutines.
	// You can configure the buffer size and concurrency through the Uploader's
	// parameters.
	Upload(input *s3manager.UploadInput, options ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error)
}

// Downloader provides an abstraction to aid in mocking for unit tests
type Downloader interface {
	// Download downloads an object in S3 and writes the payload into w using
	// concurrent GET requests. The n int64 returned is the size of the object downloaded
	// in bytes.
	Download(w io.WriterAt, input *s3.GetObjectInput, options ...func(*s3manager.Downloader)) (n int64, err error)
}

package images

//go:generate go run github.com/golang/mock/mockgen -destination mocks/reader.go github.com/itsHabib/sim/internal/images Reader
//go:generate go run github.com/golang/mock/mockgen -destination mocks/writer.go github.com/itsHabib/sim/internal/images Writer

import (
	"io"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
)

// Record represents the image record stored in the db that links to an actual
// image in cloud storage.
type Record struct {
	// ID of the record
	ID string `json:"id"`

	// CreatedAt is the created time stamp
	CreatedAt *time.Time `json:"createdAt"`

	// Etag of the object
	ETag string `json:"etag"`

	// Key of the object in cloud storage
	Key string `json:"key"`

	// Name of the object given during an upload. This must be unique.
	Name string `json:"name"`

	// Size is the size of the object in KB
	Size int64 `json:"size"`

	// Storage is the cloud storage that holds the underlying images
	// i.e. an AWS bucket
	Storage string `json:"storage"`
}

// Reader interface provides the means to read image records from the underlying
// database.
type Reader interface {
	// Get provides the means to retrieve an image record by id.
	Get(id string) (*Record, error)
	// List provides the means to list image records from the db.
	List() ([]Record, error)
}

// Writer interface provides the means to write image records to the underlying
// database.
type Writer interface {
	// Create provides the means to create image records in the db.
	Create(record *Record) error
}

// SessionGetter provides the caller a way retrieve an AWS session with
// options they provide. Added to aid mocking in unit/integration tests
type SessionGetter func() (*session.Session, error)

// WithSessionOptions provides the way to configure the session with custom
// aws config options
func WithSessionOptions(opts ...*aws.Config) SessionGetter {
	return func() (*session.Session, error) {
		return session.NewSession(opts...)
	}
}

// DownloadRequest represents the type used to request a download on an
// io.Reader to cloud storage.
type DownloadRequest struct {
	// ID of the image.
	ID string

	// Stream represents the io writer that the object will be downloaded into
	Stream io.WriterAt
}

// UploadRequest represents the type used to request an upload on an io.Reader
// to cloud storage.
type UploadRequest struct {
	// Name of the file to upload
	Name string

	// Body of the data to upload
	Body io.Reader
}

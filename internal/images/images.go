package images

import "time"

// Record represents the image record stored in the db that links to an actual
// image in cloud storage.
type Record struct {
	// ID of the record
	ID string `json:"id"`

	// CreatedAt is the created time stamp
	CreatedAt *time.Time `json:"createdAt"`

	//  TODO: not needed unless we're checking the checksum of local files,
	//   prob wont for this
	//ETag string `json:"etag"`

	// Key of the object in cloud storage
	Key string `json:"key"`

	// Storage is the cloud storage that holds the underlying images
	// i.e. an AWS bucket
	Storage string `json:"storage"`
}

// Reader interface provides the means to read image records from the underlying
// database.
type Reader interface {
	// List provides the means to list image records from the db.
	List() ([]Record, error)
}

// Writer interface provides the means to write image records to the underlying
// database.
type Writer interface {
	// Create provides the means to create image records in the db.
	Create(record *Record) error
}

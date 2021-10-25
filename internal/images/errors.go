package images

const (
	ErrRecordNotFound Error = "no image record(s) found"
	ErrObjectNotFound Error = "no object found in storage"
)

// Error provides a type to return named errors
type Error string

func (e Error) Error() string { return string(e) }

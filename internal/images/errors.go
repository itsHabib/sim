package images

const (
	ErrRecordNotFound Error = "no image record found"
)

// Error provides a type to return named errors
type Error string

func (e Error) Error() string { return string(e) }

package mongoerrors

import "fmt"

// Error is a MongoDB-style command error.
type Error struct {
	Code    int32
	Name    string
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s (%d): %s", e.Name, e.Code, e.Message)
}

// New creates a MongoDB-style error.
func New(code int32, name string, format string, args ...any) *Error {
	return &Error{
		Code:    code,
		Name:    name,
		Message: fmt.Sprintf(format, args...),
	}
}

const (
	// CodeCommandNotFound mirrors MongoDB's command-not-found class.
	CodeCommandNotFound int32 = 59
	// CodeBadValue mirrors MongoDB's bad-value class.
	CodeBadValue int32 = 2
)

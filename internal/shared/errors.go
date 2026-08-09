package shared

// Error represents a domain error with code.
type Error struct {
	Code    string
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Cause
}

// Common error codes.
const (
	CodeNotFound      = "not_found"
	CodeConflict      = "conflict"
	CodeUnauthorized  = "unauthorized"
	CodeForbidden     = "forbidden"
	CodeInternal      = "internal"
	CodeBadRequest    = "bad_request"
	CodeTimeout       = "timeout"
	CodeUnavailable   = "unavailable"
)

// NewError creates a new domain error.
func NewError(code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// WrapError wraps an existing error with domain context.
func WrapError(code, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Cause: cause}
}

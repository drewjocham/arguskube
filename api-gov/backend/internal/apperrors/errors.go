package apperrors

type Error string

func (e Error) Error() string {
	return string(e)
}

// Shared log key constants used across packages for structured logging.
const (
	LogKeyError = "error"
)

const (
	ErrSpecNotFound        Error = "spec not found"
	ErrEndpointNotFound    Error = "endpoint not found"
	ErrDriftReportNotFound Error = "drift report not found"
	ErrValidation          Error = "validation failed"
	ErrServerConfigNil     Error = "server config must not be nil"
	ErrSpecContentInvalid  Error = "spec content is invalid or unparseable"
	ErrUnknown             Error = "an unknown unexpected error occurred"
	ErrBadRequest          Error = "bad request"
)

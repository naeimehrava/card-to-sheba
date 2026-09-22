package inquiry

import "fmt"

type Kind string

const (
	KindValidation          Kind = "VALIDATION_ERROR"
	KindNotFound            Kind = "CARD_NOT_FOUND"
	KindUpstreamRejected    Kind = "UPSTREAM_REJECTED"
	KindUpstreamAuth        Kind = "UPSTREAM_AUTH_ERROR"
	KindUpstreamTimeout     Kind = "UPSTREAM_TIMEOUT"
	KindUpstreamUnavailable Kind = "UPSTREAM_UNAVAILABLE"
	KindInternal            Kind = "INTERNAL_ERROR"
)

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type Error struct {
	Kind    Kind
	Message string
	Fields  []FieldError
	cause   error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Kind, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Kind, e.Message)
}

func (e *Error) Unwrap() error { return e.cause }

func NewError(kind Kind, message string, cause error) *Error {
	return &Error{Kind: kind, Message: message, cause: cause}
}

func newValidationError(field, message string) *Error {
	return &Error{
		Kind:    KindValidation,
		Message: message,
		Fields:  []FieldError{{Field: field, Message: message}},
	}
}

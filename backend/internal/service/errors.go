package service

import (
	"fmt"
	"math"
)

type ErrorKind int

const (
	KindValidation ErrorKind = iota + 1
	KindNotFound
	KindConflict
)

// Error carries a client-safe message; handlers map Kind to an HTTP status.
// Any other error type returned by a service is treated as internal.
type Error struct {
	Kind    ErrorKind
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

func validationError(format string, args ...any) error {
	return &Error{Kind: KindValidation, Message: fmt.Sprintf(format, args...)}
}

func notFoundError(format string, args ...any) error {
	return &Error{Kind: KindNotFound, Message: fmt.Sprintf(format, args...)}
}

func conflictError(format string, args ...any) error {
	return &Error{Kind: KindConflict, Message: fmt.Sprintf(format, args...)}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// Package errors defines error handling resources used in Convo's app.
// It is based on patterns developed at Upspin:
// https://commandcenter.blogspot.com/2017/12/error-handling-in-upspin.html
package errors

import (
	"fmt"
	"net/http"
	"strings"
)

// ClientReporter provides information about an error such that client and
// server errors can be distinguished and handled appropriately.
type ClientReporter interface {
	error
	ClientReport() map[string]string
	StatusCode() int
}

// Error is the type that implements the error interface.
// It contains a number of fields, each of different type.
// An Error value may leave some values unset.
type Error struct {
	err      error
	kind     Kind
	op       Op
	messages map[string]string
}

// Kind defines the kind of error this is.
type Kind uint8

// Op describes an operation, usually as the package and method,
// such as "models.GetUser".
type Op string

// Kinds of errors.
const (
	Validation Kind = iota // Validation errors are caused by invalid parameters
	Permission
	NotFound
	Internal // Used for internal server errors
)

// E creates a new Error instance. The extras arguments can be either an error or
// a message for the client as map[string]string. If one of the extras is an error,
// its messages, if it has any, are merged into the new error's messages.
func E(op Op, kind Kind, extras ...interface{}) error {
	e := &Error{
		op:       op,
		kind:     kind,
		messages: map[string]string{},
	}

	for _, ex := range extras {
		switch t := ex.(type) {
		case ClientReporter:
			// Merge client reports. If it is attempted to write the same key more
			// than once, the later write always wins.
			for k, v := range t.ClientReport() {
				if _, has := e.messages[k]; !has {
					e.messages[k] = v
				}
			}

			// If there is more than one error, which, as a best practice, there
			// shouldn't be, the last error wins.
			e.err = t
		case error:
			e.err = t
		case map[string]string:
			// New error messages win.
			for k, v := range t {
				e.messages[k] = v
			}
		}
	}

	return e
}

// Error returns a string with information about the error for debugging purposes.
// This value should not be returned to the user.
func (e *Error) Error() string {
	b := new(strings.Builder)
	b.WriteString(fmt.Sprintf("%s:%s", string(e.op), string(e.kind)))
	if e.err != nil {
		b.WriteString(fmt.Sprintf("::%v", e.err))
	}
	return b.String()
}

// ClientReport returns a map of strings suitable to be returned to the end user.
func (e *Error) ClientReport() map[string]string {
	if len(e.messages) == 0 {
		switch e.kind {
		case Validation:
			return map[string]string{"message": "The request was invalid"}
		case Permission:
			return map[string]string{"message": "You do not have permission to perform this action"}
		case NotFound:
			return map[string]string{"message": "The requested resource was not found"}
		default:
			return map[string]string{"message": "Something went wrong"}
		}
	}

	return e.messages
}

// StatusCode returns the HTTP status code for the error.
func (e *Error) StatusCode() int {
	switch e.kind {
	case Validation:
		return http.StatusBadRequest
	case Permission:
		return http.StatusForbidden
	case NotFound:
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

// Errorf is the same things as fmt.Errorf. It is exported for convenience and so that
// this package can handle all errors.
func Errorf(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}

// Str returns an error from the given string.
func Str(s string) error {
	return fmt.Errorf(s)
}

// Package errors defines error handling resources used in Convo's app.
// It is based on patterns developed at Upspin:
// https://commandcenter.blogspot.com/2017/12/error-handling-in-upspin.html
package errors

import (
	native "errors"
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
	code     int
	op       Op
	messages map[string]string
}

// Op describes an operation, usually as the package and method,
// such as "models.GetUser".
type Op string

// E creates a new Error instance. The extras arguments can be (1) an error, (2)
// a message for the client as map[string]string, or (3) an HTTP status code. If
// one of the extras is an error that implements ClientReporter, its messages,
// if it has any, are merged into the new error's messages.
func E(op Op, extras ...interface{}) error {
	e := &Error{
		op:       op,
		code:     http.StatusInternalServerError,
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

			// If the code has already been set to something other than the default
			// don't reset it; otherwise, inherit from t.
			if e.code == http.StatusInternalServerError {
				e.code = t.StatusCode()
			}
		case int:
			e.code = t
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
	b.WriteString(fmt.Sprintf("%s: ", string(e.op)))

	if e.err != nil {
		b.WriteString(e.err.Error())
	}

	return b.String()
}

// ClientReport returns a map of strings suitable to be returned to the end user.
func (e *Error) ClientReport() map[string]string {
	if len(e.messages) == 0 {
		switch e.code {
		case http.StatusBadRequest:
			return map[string]string{"message": "The request was invalid"}
		case http.StatusUnauthorized:
			return map[string]string{"message": "Unauthorized"}
		case http.StatusForbidden:
			return map[string]string{"message": "You do not have permission to perform this action"}
		case http.StatusNotFound:
			return map[string]string{"message": "The requested resource was not found"}
		case http.StatusUnsupportedMediaType:
			return map[string]string{"message": "Unsupported content-type"}
		default:
			return map[string]string{"message": "Something went wrong"}
		}
	}

	return e.messages
}

// StatusCode returns the HTTP status code for the error.
func (e *Error) StatusCode() int {
	if e.code >= http.StatusBadRequest {
		return e.code
	}

	return http.StatusInternalServerError
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

// Opf returns an Op from the given format string.
func Opf(format string, args ...interface{}) Op {
	return Op(fmt.Sprintf(format, args...))
}

func Is(err, target error) bool {
	return native.Is(err, target)
}

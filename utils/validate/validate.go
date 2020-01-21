package validate

import (
	"fmt"
	"strings"

	"github.com/imdario/mergo"
	"github.com/microcosm-cc/bluemonday"
	"gopkg.in/validator.v2"
)

type ValidationError struct {
	errors map[string]string
}

func (v *ValidationError) Error() string {
	return fmt.Sprintf("%v", v.errors)
}

func (v *ValidationError) ToMapString() map[string]string {
	return v.errors
}

// Do validates the payload and casts it onto the given struct.
func Do(dst interface{}, payload map[string]interface{}) *ValidationError {
	// Remove whitespace from any string fields and lower case email
	cleaned := make(map[string]interface{})
	for k, v := range payload {
		if s, ok := v.(string); ok {
			if strings.ToLower(k) == "email" {
				s = strings.ToLower(s)
			}

			trimmed := strings.TrimSpace(s)
			sanitized := bluemonday.StrictPolicy().Sanitize(trimmed)
			cleaned[upperFirstLetter(k)] = sanitized
		} else {
			cleaned[upperFirstLetter(k)] = v
		}
	}

	if err := mergo.Map(dst, cleaned); err != nil {
		return &ValidationError{map[string]string{"message": err.Error()}}
	}

	if errs := validator.Validate(dst); errs != nil {
		return &ValidationError{normalizeErrors(errs)}
	}

	return nil
}

func normalizeErrors(e interface{}) map[string]string {
	normalized := make(map[string]string)

	errs := e.(validator.ErrorMap)

	for field, errs := range errs {
		err := errs[0] // Take just the first error

		switch err {
		case validator.ErrZeroValue:
			normalized[lowerFirstLetter(field)] = "This field is required"
		case validator.ErrMin:
			if field == "Password" {
				normalized[lowerFirstLetter(field)] = "Must be at least 8 characters long"
			} else {
				normalized[lowerFirstLetter(field)] = "This is too short"
			}
		case validator.ErrMax:
			normalized[lowerFirstLetter(field)] = "This is too long"
		case validator.ErrRegexp:
			if field == "Email" {
				normalized[lowerFirstLetter(field)] = "This is not a valid email"
			} else {
				normalized[lowerFirstLetter(field)] = "Nope"
			}
		default:
			normalized[lowerFirstLetter(field)] = "Nope"
		}
	}

	return normalized
}

func lowerFirstLetter(s string) string {
	if r := rune(s[0]); r >= 'A' && r <= 'Z' {
		s = strings.ToLower(string(r)) + s[1:]
	}

	if s[len(s)-2:] == "ID" {
		s = s[:len(s)-2] + "Id"
	}

	return s
}

func upperFirstLetter(s string) string {
	if r := rune(s[0]); r >= 'A' && r <= 'Z' {
		s = strings.ToUpper(string(r)) + s[1:]
	}

	if s[len(s)-2:] == "Id" {
		s = s[:len(s)-2] + "ID"
	}

	return s
}

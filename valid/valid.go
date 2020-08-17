package valid

import (
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"gopkg.in/validator.v2"

	"github.com/hiconvo/api/errors"
)

// nolint
var _emailRe = regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,14}$`)

func Email(email string) (string, error) {
	email = strings.TrimSpace(strings.ToLower(email))

	if _emailRe.MatchString(email) {
		return email, nil
	}

	return "", errors.E(
		errors.Opf("valid.Email(%q)", email),
		errors.Str("invalid email"),
		http.StatusBadRequest)
}

func URL(in string) (string, error) {
	parsed, err := url.ParseRequestURI(in)
	if err != nil {
		return "", errors.Str("invalid url")
	}

	return parsed.String(), nil
}

func Raw(in interface{}) error {
	v := reflect.ValueOf(in).Elem()

	for i := 0; i < v.NumField(); i++ {
		val, ok := v.Field(i).Interface().(string)
		if !ok {
			continue
		}

		// if it's an email address, lower case it
		if _emailRe.MatchString(strings.ToLower(val)) {
			val = strings.ToLower(val)
		}

		val = strings.TrimSpace(val)
		val = bluemonday.StrictPolicy().Sanitize(val)

		if v.Field(i).CanSet() {
			v.Field(i).SetString(val)
		}
	}

	if err := validator.Validate(in); err != nil {
		return errors.E(errors.Op("valid.Raw"), normalizeErrors(err), http.StatusBadRequest, err)
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

	if len(s) >= 2 && s[len(s)-2:] == "Id" {
		s = s[:len(s)-2] + "ID"
	}

	return s
}

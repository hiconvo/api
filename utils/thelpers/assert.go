package thelpers

import (
	"fmt"
	"net/http/httptest"
	"reflect"
	"testing"
)

func AssertEqual(t *testing.T, got, want interface{}) {
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Assert error:\ngot:\n\t%s\n\nwant:\n\t%s\n\n",
			toString(got), toString(want))
	}
}

func AssertStatusCodeEqual(t *testing.T, rr *httptest.ResponseRecorder, want int) {
	got := rr.Code

	if got != want {
		if rr.Body != nil {
			t.Errorf("handler returned wrong status code: got %v want %v\n\nresponse body:\n\t%s\n\n",
				got, want, rr.Body.String())
		} else {
			t.Errorf("handler returned wrong status code: got %v want %v",
				got, want)
		}
	}
}

func AssetObjectsContainKeys(t *testing.T, key string, wantedValues []string, got []interface{}) {
	// Get all of the key values into a slice
	gotValues := make([]string, len(got))
	for i, item := range got {
		obj := item.(map[string]interface{})
		gotValue := obj[key].(string)
		gotValues[i] = gotValue
	}

	for _, gotValue := range gotValues {
		contains := false
		for _, wantedValue := range wantedValues {
			if gotValue == wantedValue {
				contains = true
				break
			}
		}

		if !contains {
			t.Errorf("handler returned unexpected value for '%s': got %v want any of %v",
				key, gotValue, wantedValues)
		}
	}

	for _, wantedValue := range wantedValues {
		contains := false
		for _, gotValue := range gotValues {
			if gotValue == wantedValue {
				contains = true
				break
			}
		}

		if !contains {
			t.Errorf("handler did not return expected value for key '%s': want %v", key, wantedValue)
		}
	}
}

func AssetObjectsNOTContainKeys(t *testing.T, key string, got []interface{}) {
	defer func() { recover() }()
	for _, item := range got {
		obj := item.(map[string]interface{})
		gotKey := obj[key].(string)
		if gotKey == key {
			t.Errorf("handler returned unwanted key: got %v",
				gotKey)
		}
	}
}

func toString(v interface{}) string {
	return fmt.Sprintf("%v", v)
}

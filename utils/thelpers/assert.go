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

func AssertObjectsContainIDs(t *testing.T, got []interface{}, wantedIDs []string) {
	gotIDs := make([]string, len(got))
	for i, c := range got {
		typedC := c.(map[string]interface{})
		id := typedC["id"].(string)
		gotIDs[i] = id
	}

	for _, gotID := range gotIDs {
		contains := false
		for _, wantedID := range wantedIDs {
			if gotID == wantedID {
				contains = true
			}
		}

		if !contains {
			t.Errorf("handler returned unexpected id: got %v want any of %v",
				gotID, wantedIDs)
		}
	}

	for _, wantedID := range wantedIDs {
		contains := false
		for _, gotID := range gotIDs {
			if gotID == wantedID {
				contains = true
			}
		}

		if !contains {
			t.Errorf("handler did not return expected id: want %v", wantedID)
		}
	}
}

func toString(v interface{}) string {
	return fmt.Sprintf("%v", v)
}

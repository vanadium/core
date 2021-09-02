package security_test

import (
	"reflect"
	"testing"
	"time"

	"v.io/v23/security"
	"v.io/v23/vdl"
)

func TestCopyParams(t *testing.T) {
	when := time.Now()
	cpy := &security.CallParams{}
	orig := &security.CallParams{
		Timestamp: when,
		Method:    "method",
	}
	call := security.NewCall(orig)
	cpy.Copy(call)
	if got, want := cpy, orig; !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
	orig = &security.CallParams{
		Timestamp:       when,
		Method:          "method",
		MethodTags:      []*vdl.Value{vdl.StringValue(nil, "oops")},
		Suffix:          "/",
		LocalDischarges: map[string]security.Discharge{"dc": {}},
	}
	call = security.NewCall(orig)
	cpy = &security.CallParams{}
	cpy.Copy(call)
	if got, want := cpy, orig; !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

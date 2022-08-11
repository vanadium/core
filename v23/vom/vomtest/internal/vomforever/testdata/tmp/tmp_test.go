package tmp

import (
	"testing"

	"v.io/v23/vdl"
	"v.io/v23/vom"
)

func TestEncodeDecode(t *testing.T) {
	bytes, err := vom.Encode(X)
	if err != nil {
		t.Errorf("error in encode: %v", err)
	}
	var out XType
	if err := vom.Decode(bytes, &out); err != nil {
		t.Fatalf("error in decode: %v", err)
	}
	if !vdl.DeepEqual(out, X) {
		t.Errorf("got: %#v, want: %#v", out, X)
	}
}
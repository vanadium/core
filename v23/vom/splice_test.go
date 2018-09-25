package vom_test

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"v.io/v23/vom"
)

func ExampleAlreadyEncodedBytes() {

	type m struct {
		M string
	}
	abuf := &bytes.Buffer{}
	encA := vom.NewEncoder(abuf)
	encA.Encode(&m{"stream-a"})

	bbuf := &bytes.Buffer{}
	encB := vom.NewEncoder(bbuf)
	encB.Encode(m{"stream-b"})
	encB.Encode(vom.AlreadyEncodedBytes{abuf.Bytes()})

	dec := vom.NewDecoder(bbuf)
	va, vb := m{}, m{}
	dec.Decode(&vb)
	dec.Decode(&va)
	fmt.Println(va.M)
	fmt.Println(vb.M)
	// Output:
	// stream-a
	// stream-b
}

func TestSplice(t *testing.T) {

	type tt1 struct {
		Dict map[string]bool
	}
	type tt2 struct {
		Name string
	}
	type tt3 int

	v0 := true
	v1 := tt1{map[string]bool{"t": true, "f": false}}
	v2 := tt2{"simple-test"}
	v3 := tt3(42)

	buf := &bytes.Buffer{}
	enc := vom.NewEncoder(buf)
	for _, v := range []interface{}{v1, v2, v3} {
		if err := enc.Encode(v); err != nil {
			t.Fatal(err)
		}
	}

	// Re-encod & write out the pre-encoded bytes from above, the result
	// should be the same byte stream.
	tbuf := &bytes.Buffer{}
	enc = vom.NewEncoder(tbuf)
	if err := enc.Encode(v0); err != nil {
		t.Fatal(err)
	}

	if err := enc.Encode(vom.AlreadyEncodedBytes{buf.Bytes()}); err != nil {
		t.Fatal(err)
	}

	var (
		r0 bool
		r1 tt1
		r2 tt2
		r3 tt3
	)

	dec := vom.NewDecoder(tbuf)
	if err := dec.Decode(&r0); err != nil {
		t.Fatal(err)
	}
	if err := dec.Decode(&r1); err != nil {
		t.Fatal(err)
	}
	if err := dec.Decode(&r2); err != nil {
		t.Fatal(err)
	}
	if err := dec.Decode(&r3); err != nil {
		t.Fatal(err)
	}
	if got, want := r0, v0; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := r1, v1; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := r2, v2; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := r3, v3; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

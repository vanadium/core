// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package exec

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"unicode/utf8"

	"v.io/v23/verror"
)

func checkPresent(t *testing.T, c Config, k, wantV string) {
	if v, err := c.Get(k); err != nil {
		t.Errorf("Expected value %q for key %q, got error %v instead", wantV, k, err)
	} else if v != wantV {
		t.Errorf("Expected value %q for key %q, got %q instead", wantV, k, v)
	}
}

func checkAbsent(t *testing.T, c Config, k string) {
	if v, err := c.Get(k); !errors.Is(err, verror.ErrNoExist) {
		t.Errorf("Expected (\"\", %v) for key %q, got (%q, %v) instead", verror.ErrNoExist, k, v, err)
	}
}

// TestConfig checks that Set and Get work as expected.
func TestConfig(t *testing.T) {
	c := NewConfig()
	c.Set("foo", "bar")
	checkPresent(t, c, "foo", "bar")
	checkAbsent(t, c, "food")
	c.Set("foo", "baz")
	checkPresent(t, c, "foo", "baz")
	c.Clear("foo")
	checkAbsent(t, c, "foo")
	if want, got := map[string]string{}, c.Dump(); !reflect.DeepEqual(want, got) {
		t.Errorf("Expected %v for Dump, got %v instead", want, got)
	}
}

func vomCodec(o, n Config) error {
	s, err := o.Serialize()
	if err != nil {
		return err
	}
	if err := n.MergeFrom(s); err != nil {
		return err
	}
	return nil
}

// TestSerialize checks that serializing the config and merging from a
// serialized config work as expected.
func TestSerialize(t *testing.T) {
	testCodec(t, vomCodec)
}

func jsonCodec(o, n Config) error {
	s, err := json.Marshal(o)
	if err != nil {
		return err
	}
	return json.Unmarshal(s, n)
}

// TestSerializeJSON checks that serializing the config and merging from a
// serialized JSON config work as expected.
func TestSerializedJSON(t *testing.T) {
	testCodec(t, jsonCodec)
}

func testCodec(t *testing.T, fn func(o, n Config) error) {
	c := NewConfig()
	c.Set("k1", "v1")
	c.Set("k2", "v2")

	readC := NewConfig()
	err := fn(c, readC)
	if err != nil {
		t.Fatalf("Failed to serialize/deserialize: %v", err)
	}

	checkPresent(t, readC, "k1", "v1")
	checkPresent(t, readC, "k2", "v2")

	readC.Set("k2", "newv2") // This should be overwritten by the next merge.
	checkPresent(t, readC, "k2", "newv2")
	readC.Set("k3", "v3") // This should survive the next merge.

	c.Set("k1", "newv1") // This should overwrite v1 in the next merge.
	c.Set("k4", "v4")    // This should be added following the next merge.

	err = fn(c, readC)
	if err != nil {
		t.Fatalf("Failed to serialize/deserialize: %v", err)
	}

	checkPresent(t, readC, "k1", "newv1")
	checkPresent(t, readC, "k2", "v2")
	checkPresent(t, readC, "k3", "v3")
	checkPresent(t, readC, "k4", "v4")
	if want, got := map[string]string{"k1": "newv1", "k2": "v2", "k3": "v3", "k4": "v4"}, readC.Dump(); !reflect.DeepEqual(want, got) {
		t.Errorf("Expected %v for Dump, got %v instead", want, got)
	}
}

func isbase64(s string) bool {
	for _, r := range s {
		if utf8.RuneLen(r) != 1 {
			return false
		}
		switch {
		case r >= 65 && r <= 90: // A-Z
			continue
		case r >= 97 && r <= 122: // a-z
			continue
		case r >= 48 && r <= 57: // 0-9
			continue
		case r == 43: // +
			continue
		case r == 47: // /
			continue
		case r == 61: // =
			continue
		default:
			return false
		}
	}
	return true
}

func TestEnvVar(t *testing.T) {
	c := NewConfig()
	c.Set("k1", "v1")
	c.Set("k2", "v2")

	val, err := EncodeForEnvVar(c)
	if err != nil {
		t.Fatal(err)
	}
	if !isbase64(val) {
		t.Fatalf("%v contains a non-base64 character as per RFC 4648", val)
	}

	n := NewConfig()
	err = DecodeFromEnvVar(val, n)
	if err != nil {
		t.Fatal(err)
	}
	v, err := n.Get("k1")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := v, "v1"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

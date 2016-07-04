// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"testing"
)

func TestObject(t *testing.T) {
	o := make(object)
	o.set("foo", "bar")
	o.set("slice", []interface{}{"a", "b", "c"})
	o.set("obj", object{"name": "Bob"})
	o.set("x.y.z", 5)
	o.append("slice", "d")

	out, err := o.json()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expected := `{
  "foo": "bar",
  "obj": {
    "name": "Bob"
  },
  "slice": [
    "a",
    "b",
    "c",
    "d"
  ],
  "x": {
    "y": {
      "z": 5
    }
  }
}`
	if got := string(out); got != expected {
		t.Errorf("Unexpected output. Got %q, expected %q", got, expected)
	}
}

func TestObjectJSON(t *testing.T) {
	json := `{
          "foo": "bar",
          "bar": 10,
          "list": [ { "x":0 }, { "x":1 }, { "x":2 } ],
          "x": { "y": [ 1, 2, 3 ] }
        }`

	o := make(object)
	if err := o.importJSON([]byte(json)); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if got, expected := o.getString("foo"), "bar"; got != expected {
		t.Errorf("Unexpected value. Got %#v, expected %#v", got, expected)
	}
	if got, expected := o.getInt("bar", -1), 10; got != expected {
		t.Errorf("Unexpected value. Got %#v, expected %#v", got, expected)
	}
	if got, expected := o.getString("notthere"), ""; got != expected {
		t.Errorf("Unexpected value. Got %#v, expected %#v", got, expected)
	}
	if got, expected := o.getString("x.y"), "[1 2 3]"; got != expected {
		t.Errorf("Unexpected value. Got %#v, expected %#v", got, expected)
	}
	o.append("x.y", 4)
	list := o.getObjectArray("list")
	for i, item := range list {
		if got, expected := item.get("x"), float64(i); got != expected {
			t.Errorf("Unexpected value for x. Got %#v, expected %#v", got, expected)
		}
	}
	list = append(list, object{"x": "y"})
	o.set("list", list)

	out, err := o.json()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expected := `{
  "bar": 10,
  "foo": "bar",
  "list": [
    {
      "x": 0
    },
    {
      "x": 1
    },
    {
      "x": 2
    },
    {
      "x": "y"
    }
  ],
  "x": {
    "y": [
      1,
      2,
      3,
      4
    ]
  }
}`
	if got := string(out); got != expected {
		t.Errorf("Unexpected output. Got %q, expected %q", got, expected)
	}
}

// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// object simplifies the parsing and handling of json objects that are
// unmarshaled into an empty interface.
type object map[string]interface{}

func (o *object) importJSON(data []byte) error {
	var decode interface{}
	if err := json.Unmarshal(data, &decode); err != nil {
		return err
	}
	c := convertToObject(decode)
	var ok bool
	if *o, ok = c.(object); !ok {
		return fmt.Errorf("object is %T", c)
	}
	return nil
}

// convertToObject converts all occurrences of map[string]interface{} to object.
func convertToObject(i interface{}) interface{} {
	switch obj := i.(type) {
	case map[string]interface{}:
		for k, v := range obj {
			obj[k] = convertToObject(v)
		}
		return object(obj)
	case []interface{}:
		for x, y := range obj {
			obj[x] = convertToObject(y)
		}
		return obj
	default:
		return obj
	}
}

func (o object) json() ([]byte, error) {
	return json.MarshalIndent(o, "", "  ")
}

// get retrieves the value of an object inside this object, e.g.:
// if o = { "a": { "b": "c" } }, o.get("a.b") == "c".
func (o object) get(name string) interface{} {
	parts := strings.Split(name, ".")
	var obj interface{} = o
	for _, p := range parts {
		m, ok := obj.(object)
		if !ok {
			return nil
		}
		var exists bool
		if obj, exists = m[p]; !exists {
			return nil
		}
	}
	return obj
}

// set sets the value of an object inside this object, e.g.:
// if o = { "a": { "b": "c" } }, o.set("a.b", "X") change "c" to "X".
func (o object) set(name string, value interface{}) error {
	parts := strings.Split(name, ".")
	var obj interface{} = o
	for {
		m, ok := obj.(object)
		if !ok {
			return fmt.Errorf("%q not an object", name)
		}

		p := parts[0]
		parts = parts[1:]

		if len(parts) == 0 {
			m[p] = value
			break
		}
		if obj, ok = m[p]; !ok {
			obj = make(object)
			m[p] = obj
		}
	}
	return nil
}

// getString retrieves a string object.
func (c object) getString(name string) string {
	switch s := c.get(name).(type) {
	case string:
		return s
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", s)
	}
}

// getString retrieves a integer object.
func (c object) getInt(name string, defaultValue int) int {
	switch v := c.get(name).(type) {
	case int:
		return v
	case float64:
		return int(v)
	default:
		return defaultValue
	}
}

// getObjectArray retrieves an array of objects.
func (c object) getObjectArray(name string) []object {
	s, ok := c.get(name).([]interface{})
	if !ok {
		return nil
	}
	n := make([]object, len(s))
	for i, o := range s {
		if x, ok := o.(object); ok {
			n[i] = x
			continue
		}
		return nil
	}
	return n
}

// append adds objects to an array.
func (c object) append(name string, values ...interface{}) error {
	obj := c.get(name)
	if obj == nil {
		obj = []interface{}{}
	}
	switch array := obj.(type) {
	case []interface{}:
		return c.set(name, append(array, values...))
	default:
		return fmt.Errorf("%q is not an array", name)
	}
}

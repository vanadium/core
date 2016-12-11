// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pubsub

import (
	"fmt"

	"time"
)

// Format formats a Setting in a consistent manner, it is intended to be
// used when implementing the Setting interface.
func Format(s Setting) string {
	return fmt.Sprintf("%s: %s: (%T: %s)", s.Name(), s.Description(), s.Value(), s.Value())
}

// Type Any can be used to represent or implement a Setting of any type.
type Any struct {
	name, description string
	value             interface{}
}

func (s *Any) String() string {
	return Format(s)
}

func (s *Any) Name() string {
	return s.name
}

func (s *Any) Description() string {
	return s.description
}

func (s *Any) Value() interface{} {
	return s.value
}

func NewAny(name, description string, value interface{}) Setting {
	return &Any{name, description, value}
}

func NewInt(name, description string, value int) Setting {
	return &Any{name, description, value}
}

func NewInt64(name, description string, value int64) Setting {
	return &Any{name, description, value}
}

func NewBool(name, description string, value bool) Setting {
	return &Any{name, description, value}
}

func NewFloat64(name, description string, value float64) Setting {
	return &Any{name, description, value}
}

func NewString(name, description string, value string) Setting {
	return &Any{name, description, value}
}

func NewDuration(name, description string, value time.Duration) Setting {
	return &Any{name, description, value}
}

// DurationFlag implements flag.Value in order to provide validation of
// duration values in the flag package.
type DurationFlag struct{ time.Duration }

// Implements flag.Value.Get
func (d DurationFlag) Get() interface{} {
	return d.Duration
}

// Implements flag.Value.Set
func (d *DurationFlag) Set(s string) error {
	duration, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = duration
	return nil
}

// Implements flag.Value.String
func (d DurationFlag) String() string {
	return d.Duration.String()
}

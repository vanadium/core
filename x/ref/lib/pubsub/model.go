// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package pubsub defines interfaces for accessing dynamically changing
// process configuration information.
//
// Settings represent configuration parameters and their value. Settings
// are published to named Streams. Streams are forked to add additional
// consumers, i.e. readers of the Settings published to the Stream.
//
// Settings are represented by an interface type that wraps the data and
// provides a name and description for each Settings. Streams are similarly
// named and also have a description. When streams are 'forked' the latest
// value of all Settings that have been sent over the Stream are made
// available to the caller. This allows for a rendezvous between the single
// producer of Settings and multiple consumers of those Settings that
// may be added at arbitrary points in time.
//
// Streams are hosted by a Publisher type, which in addition to the methods
// required for managing Streams provides a means to shut down all of the
// Streams it hosts.
package pubsub

// Setting must be implemented by all data types to sent over Publisher
// streams.
type Setting interface {
	String() string
	// Name returns the name of the Setting
	Name() string
	// Description returns the description of the Setting
	Description() string
	// Value returns the value of the Setting.
	Value() interface{}
}

// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package concurrency

// resourceKey represents an identifier of an abstract resource.
type resourceKey interface{}

// resourceSet represents a set of abstract resources.
type resourceSet map[resourceKey]struct{}

// newResourceSet if the resourceSet factory.
func newResourceSet() resourceSet {
	return make(resourceSet)
}

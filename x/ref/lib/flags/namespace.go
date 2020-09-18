// Copyright 2018 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags

import "fmt"

// NamespaceRootFlag represents a flag.Value for --v23.namespace.root.
type NamespaceRootFlag struct {
	isSet bool // is true when a flag has been explicitly set.
	// isDefault is true when a flag has the default value and is needed in
	// addition to isSet to distinguish between using a default value
	// as opposed to one from an environment variable.
	isDefault bool
	Roots     []string
}

// String implements flag.Value.
func (nsr *NamespaceRootFlag) String() string {
	return fmt.Sprintf("%v", nsr.Roots)
}

// Set implements flag.Value
func (nsr *NamespaceRootFlag) Set(v string) error {
	nsr.isDefault = false
	if !nsr.isSet {
		// Override the default value otherwise the new values would be
		// appended to the default ones rather than replacing them.
		nsr.isSet = true
		nsr.Roots = []string{}
	}
	for _, t := range nsr.Roots {
		if v == t {
			return nil
		}
	}
	nsr.Roots = append(nsr.Roots, v)
	return nil
}

// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase

import (
	"fmt"
	"strings"
)

// Note that "," is not allowed to appear in blessing patterns. We also
// could've used "/" as a separator, but then we would've had to be more
// careful with decoding and splitting name components elsewhere.
// TODO(sadovsky): Maybe define "," constant in v23/services/syncbase.

// String implements Stringer.
func (id Id) String() string {
	return id.Blessing + "," + id.Name
}

// ParseId parses an Id from a string formatted according to the String method.
func ParseId(from string) (Id, error) {
	parts := strings.SplitN(from, ",", 2)
	if len(parts) != 2 {
		return Id{}, fmt.Errorf("failed to parse id: %s", from)
	}

	return Id{Blessing: parts[0], Name: parts[1]}, nil
}

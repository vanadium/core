// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package audit implements a mechanism for writing auditable events to an audit
// log.
//
// Typical use would be for tracking sensitive operations like private key usage
// (NewPrincipal), or sensitive RPC method invocations.
package audit

import (
	"fmt"
	"strings"
	"time"

	"v.io/v23/context"
)

// Auditor is the interface for writing auditable events.
type Auditor interface {
	Audit(ctx *context.T, entry Entry) error
}

// Entry is the information logged on each auditable event.
type Entry struct {
	// Method being invoked.
	Method string
	// Arguments to the method.
	// Any sensitive data in the arguments should not be included,
	// even if the argument was provided to the real method invocation.
	Arguments []interface{}
	// Result of the method invocation.
	// A common use case is to audit only successful method invocations.
	Results []interface{}

	// Timestamp of method invocation.
	Timestamp time.Time
}

func (e Entry) String() string {
	return fmt.Sprintf("%v: %s(%s)%s", e.Timestamp.Format(time.RFC3339), e.Method, join(e.Arguments, "", ""), join(e.Results, " = (", ")"))
}

func join(elems []interface{}, prefix, suffix string) string {
	switch len(elems) {
	case 0:
		return ""
	case 1:
		return fmt.Sprintf("%s%v%s", prefix, elems[0], suffix)
	}
	strs := make([]string, len(elems))
	for i, e := range elems {
		strs[i] = fmt.Sprintf("%v", e)
	}
	return prefix + strings.Join(strs, ", ") + suffix
}

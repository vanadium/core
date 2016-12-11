// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package audit_test

import (
	"testing"
	"time"

	"v.io/x/ref/lib/security/audit"
)

func TestEntryString(t *testing.T) {
	timestamp, err := time.Parse(time.RFC3339, "2014-08-08T14:39:25-07:00") // "2014-08-08 12:56:40.698493437 -0700 PDT")
	if err != nil {
		t.Fatal(err)
	}
	var (
		onearg      = []interface{}{"Arg"}
		manyargs    = []interface{}{"Arg1", "Arg2"}
		oneresult   = []interface{}{"Result"}
		manyresults = []interface{}{"Res1", "Res2"}
		tests       = []struct {
			Entry  audit.Entry
			String string
		}{
			// No results, 0, 1 or multiple arguments
			{audit.Entry{Method: "M"}, "2014-08-08T14:39:25-07:00: M()"},
			{audit.Entry{Method: "M", Arguments: onearg}, "2014-08-08T14:39:25-07:00: M(Arg)"},
			{audit.Entry{Method: "M", Arguments: manyargs}, "2014-08-08T14:39:25-07:00: M(Arg1, Arg2)"},
			// 1 result, 0, 1 or multiple arguments
			{audit.Entry{Method: "M", Results: oneresult}, "2014-08-08T14:39:25-07:00: M() = (Result)"},
			{audit.Entry{Method: "M", Arguments: onearg, Results: oneresult}, "2014-08-08T14:39:25-07:00: M(Arg) = (Result)"},
			{audit.Entry{Method: "M", Arguments: manyargs, Results: oneresult}, "2014-08-08T14:39:25-07:00: M(Arg1, Arg2) = (Result)"},
			// Multiple results, 0, 1 or multiple arguments
			{audit.Entry{Method: "M", Results: manyresults}, "2014-08-08T14:39:25-07:00: M() = (Res1, Res2)"},
			{audit.Entry{Method: "M", Arguments: onearg, Results: manyresults}, "2014-08-08T14:39:25-07:00: M(Arg) = (Res1, Res2)"},
			{audit.Entry{Method: "M", Arguments: manyargs, Results: manyresults}, "2014-08-08T14:39:25-07:00: M(Arg1, Arg2) = (Res1, Res2)"},
		}
	)

	for _, test := range tests {
		test.Entry.Timestamp = timestamp
		if got, want := test.Entry.String(), test.String; got != want {
			t.Errorf("Got %q want %q for [%#v]", got, want, test.Entry)
		}
	}
}

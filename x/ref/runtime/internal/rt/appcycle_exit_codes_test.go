// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rt_test

import (
	appcyclelib "v.io/x/ref/runtime/internal/lib/appcycle"
)

const (
	testUnhandledStopExitCode = 10
	testForceStopExitCode     = 20
)

func init() {
	appcyclelib.UnhandledStopExitCode = testUnhandledStopExitCode
	appcyclelib.ForceStopExitCode = testForceStopExitCode
}

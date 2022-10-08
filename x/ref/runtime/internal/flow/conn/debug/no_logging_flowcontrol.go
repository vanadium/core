// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !debug
// +build !debug

package debug

func FlowControl(format string, args ...interface{}) {}

func FormatCounters(counters map[uint64]uint64) string {
	return ""
}

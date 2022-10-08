// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !debug
// +build !debug

package debug

type CallerCircularList struct{}

func (cl *CallerCircularList) Append(record string) {}

func (cl *CallerCircularList) String() string {
	return ""
}

func (cl *CallerCircularList) DumpAndExit(msg string) {}

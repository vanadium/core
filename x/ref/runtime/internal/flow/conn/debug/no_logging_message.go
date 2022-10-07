// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !debug
// +build !debug

package debug

import (
	"v.io/v23/flow/message"
)

func FormatMessage(m message.Message) string {
	return ""
}

func MessagePipe(format string, args ...interface{}) {}

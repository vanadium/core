// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !debug

package debug

func Callers(skip, limit int) []uintptr {
	return nil
}

func FormatFramesFunctionsOnly(stack []uintptr, exclusions []string) string {
	return ""
}

// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package internal defines common types and functions used by both tunnel
// clients and servers.
package internal

// Winsize defines the window size used by ioctl TIOCGWINSZ and TIOCSWINSZ.
type Winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
}

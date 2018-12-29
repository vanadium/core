// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sysinit provides config generation for a variety of platforms and
// "init" systems such as upstart, systemd etc. It is intended purely for
// bootstrapping into the Vanadium system proper.
package sysinit

// InstallSystemInit defines the interface that all configs must implement.
type InstallSystemInit interface {
	Print() error
	Install() error
	Uninstall() error
	Start() error
	Stop() error
}

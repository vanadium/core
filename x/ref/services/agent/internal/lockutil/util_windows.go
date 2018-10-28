// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build windows

package lockutil

import "github.com/pkg/errors"

func CreateLockFile(dir, file string) (string, error) {
	return "", errors.New("not implemented")
}

func StillHeld(info []byte) (bool, error) {
	return false, errors.New("not implemented")
}
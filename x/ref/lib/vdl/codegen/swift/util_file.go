// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package swift

import (
	"errors"
	"path/filepath"
	"strings"
)

func (ctx *swiftContext) vdlRootForFilePath(absolutePath string) (string, error) {
	if !strings.HasPrefix(absolutePath, "/") {
		return "", errors.New("Path not absolute or unsupported file system")
	}
	dir := filepath.Dir(absolutePath)
	for {
		if ctx.srcDirs[dir] {
			return dir, nil
		} else if dir == "/" || dir == "" {
			return "", errors.New("Hit the root before any vdl paths were found")
		}
		// Go up a level
		s := strings.Split(dir, "/")
		s = s[:len(s)-1]
		dir = "/" + filepath.Join(s...)
	}
}

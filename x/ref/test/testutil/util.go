// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testutil

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
)

// FormatLogLine will prepend the file and line number of the caller
// at the specificied depth (as per runtime.Caller) to the supplied
// format and args and return a formatted string. It is useful when
// implementing functions that factor out error handling and reporting
// in tests.
func FormatLogLine(depth int, format string, args ...interface{}) string {
	_, file, line, _ := runtime.Caller(depth)
	nargs := []interface{}{filepath.Base(file), line}
	nargs = append(nargs, args...)
	return fmt.Sprintf("%s:%d: "+format, nargs...)
}

// FileTreeOpts contains options controlling how FileTreeEqual is performed.
type FileTreeOpts struct {
	// Debug is written to with additional information describing trees that are
	// not equal.
	Debug io.Writer

	// The pathname of regular files must match File{A,B} if they are provided,
	// otherwise the file is filtered out.  Similarly the pathname of directories
	// must match Dir{A,B} if they are provided.  {File,Dir}A matches pathnames
	// found under aRoot, while {File,Dir}B matches under bRoot.
	FileA, DirA *regexp.Regexp
	FileB, DirB *regexp.Regexp
}

func (opts FileTreeOpts) debug(format string, args ...interface{}) {
	if opts.Debug != nil {
		fmt.Fprintf(opts.Debug, format, args...)
	}
}

// FileTreeEqual returns true if the filesystem trees rooted at aRoot and bRoot
// are the same.  Use opts to control how matching is performed.
func FileTreeEqual(aRoot, bRoot string, opts FileTreeOpts) (bool, error) { //nolint:gocyclo
	type node struct {
		path string
		info os.FileInfo
	}
	// Walk trees to collect nodes.
	var aNodes, bNodes []node
	nodes, file, dir := &aNodes, opts.FileA, opts.DirA
	collectFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("walk failed: %v", err)
		}
		if info.IsDir() {
			if dir != nil && !dir.MatchString(path) {
				return nil
			}
		} else {
			if file != nil && !file.MatchString(path) {
				return nil
			}
		}
		*nodes = append(*nodes, node{path, info})
		return nil
	}
	if err := filepath.Walk(aRoot, collectFn); err != nil {
		return false, err
	}
	nodes, file, dir = &bNodes, opts.FileB, opts.DirB
	if err := filepath.Walk(bRoot, collectFn); err != nil {
		return false, err
	}
	// Compare nodes.
	if len(aNodes) != len(bNodes) {
		opts.debug("node count mismatch: %v, %v", aNodes, bNodes)
		return false, nil
	}
	for ix := 0; ix < len(aNodes); ix++ {
		aNode, bNode := aNodes[ix], bNodes[ix]
		aRel, err := filepath.Rel(aRoot, aNode.path)
		if err != nil {
			return false, fmt.Errorf("Rel(%v, %v) failed: %v", aRoot, aNode.path, err)
		}
		bRel, err := filepath.Rel(bRoot, bNode.path)
		if err != nil {
			return false, fmt.Errorf("Rel(%v, %v) failed: %v", bRoot, bNode.path, err)
		}
		if aRel != bRel {
			opts.debug("%v relative path doesn't match %v", aRel, bRel)
			return false, nil
		}
		aDir, bDir := aNode.info.IsDir(), bNode.info.IsDir()
		if aDir != bDir {
			opts.debug("%v isdir:%v, but %v isdir:%v", aNode.path, aDir, bNode.path, bDir)
			return false, nil
		}
		if aDir {
			continue
		}
		aBytes, err := ioutil.ReadFile(aNode.path)
		if err != nil {
			return false, fmt.Errorf("%v read failed: %v", aNode.path, err)
		}
		bBytes, err := ioutil.ReadFile(bNode.path)
		if err != nil {
			return false, fmt.Errorf("%v read failed: %v", bNode.path, err)
		}
		if !bytes.Equal(aBytes, bBytes) {
			opts.debug("%v bytes don't match", aRel)
			return false, nil
		}
	}
	return true, nil
}

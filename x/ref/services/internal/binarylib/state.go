// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package binarylib

import (
	"crypto/md5"
	"encoding/hex"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"v.io/v23/verror"
)

var (
	errUnexpectedDepth   = verror.Register(pkgPath+".errUnexpectedDepth", verror.NoRetry, "{1:}{2:} Unexpected depth, expected a value between {3} and {4}, got {5}{:_}")
	errStatFailed        = verror.Register(pkgPath+".errStatFailed", verror.NoRetry, "{1:}{2:} Stat({3}) failed{:_}")
	errReadFileFailed    = verror.Register(pkgPath+".errReadFileFailed", verror.NoRetry, "{1:}{2:} ReadFile({3}) failed{:_}")
	errUnexpectedVersion = verror.Register(pkgPath+".errUnexpectedVersion", verror.NoRetry, "{1:}{2:} Unexpected version: expected {3}, got {4}{:_}")
)

// state holds the state shared across different binary repository
// invocations.
type state struct {
	// depth determines the depth of the directory hierarchy that the
	// binary repository uses to organize binaries in the local file
	// system. There is a trade-off here: smaller values lead to faster
	// access, while higher values allow the performance to scale to
	// larger collections of binaries. The number should be a value
	// between 0 and (md5.Size - 1).
	//
	// Note that the cardinality of each level (except the leaf level)
	// is at most 256. If you expect to have X total binary items, and
	// you want to limit directories to at most Y entries (because of
	// filesystem limitations), then you should set depth to at least
	// log_256(X/Y). For example, using hierarchyDepth = 3 with a local
	// filesystem that can handle up to 1,000 entries per directory
	// before its performance degrades allows the binary repository to
	// store 16B objects.
	depth int
	// rootDir identifies the local filesystem directory in which the
	// binary repository stores its objects.
	rootDir string
	// rootURL identifies the root URL of the HTTP server serving
	// the download URLs.
	rootURL string
}

// NewState creates a new state object for the binary service.  This
// should be passed into both NewDispatcher and NewHTTPRoot.
func NewState(rootDir, rootURL string, depth int) (*state, error) {
	if min, max := 0, md5.Size-1; min > depth || depth > max {
		return nil, verror.New(errUnexpectedDepth, nil, min, max, depth)
	}
	if _, err := os.Stat(rootDir); err != nil {
		return nil, verror.New(errStatFailed, nil, rootDir, err)
	}
	path := filepath.Join(rootDir, VersionFile)
	output, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, verror.New(errReadFileFailed, nil, path, err)
	}
	if expected, got := Version, strings.TrimSpace(string(output)); expected != got {
		return nil, verror.New(errUnexpectedVersion, nil, expected, got)
	}
	return &state{
		depth:   depth,
		rootDir: rootDir,
		rootURL: rootURL,
	}, nil
}

// dir generates the local filesystem path for the binary identified by suffix.
func (s *state) dir(suffix string) string {
	h := md5.New()
	h.Write([]byte(suffix))
	hash := hex.EncodeToString(h.Sum(nil))
	dir := ""
	for j := 0; j < s.depth; j++ {
		dir = filepath.Join(dir, hash[j*2:(j+1)*2])
	}
	return filepath.Join(s.rootDir, dir, hash)
}

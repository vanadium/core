// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package binarylib

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"v.io/v23/context"
	"v.io/v23/verror"
	"v.io/x/ref/services/internal/multipart"
)

// NewHTTPRoot returns an implementation of http.FileSystem that can be used
// to serve the content in the binary service.
func NewHTTPRoot(ctx *context.T, state *state) http.FileSystem {
	return &httpRoot{ctx: ctx, state: state}
}

type httpRoot struct {
	ctx   *context.T
	state *state
}

// TODO(caprita): Tie this in with DownloadUrl, to control which binaries
// are downloadable via url.

// Open implements http.FileSystem.  It uses the multipart file implementation
// to wrap the content parts into one logical file.
func (r httpRoot) Open(name string) (http.File, error) {
	name = strings.TrimPrefix(name, "/")
	r.ctx.Infof("HTTP handler opening %s", name)
	parts, err := getParts(r.ctx, r.state.dir(name))
	if err != nil {
		return nil, err
	}
	partFiles := make([]*os.File, len(parts))
	for i, part := range parts {
		if err := checksumExists(r.ctx, part); err != nil {
			return nil, err
		}
		dataPath := filepath.Join(part, dataFileName)
		var err error
		if partFiles[i], err = os.Open(dataPath); err != nil {
			r.ctx.Errorf("Open(%v) failed: %v", dataPath, err)
			return nil, verror.New(ErrOperationFailed, nil, dataPath)
		}
	}
	return multipart.NewFile(name, partFiles)
}

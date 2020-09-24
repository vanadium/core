// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/x/ref/services/ben"
	"v.io/x/ref/services/ben/archive"
)

// NewArchiver returns an archive.BenchmarkArchiver server that uses
// store to persist data and provides a UI to browse archived benchmark
// results at url.
func NewArchiver(store Store, url string) archive.BenchmarkArchiverServerStub {
	return archive.BenchmarkArchiverServer(&server{
		url:   url,
		store: store,
	})
}

// Authorizer implements an authorization policy that authorizes callers with
// any recognizeable blessing name.
func Authorizer() security.Authorizer { return authorizer{} }

type server struct {
	url   string
	store Store
}

func (s *server) Archive(ctx *context.T, call rpc.ServerCall, scenario ben.Scenario, code ben.SourceCode, runs []ben.Run) (string, error) {
	// Various fields must be set.
	if len(scenario.Cpu.Architecture) == 0 {
		return "", fmt.Errorf("Cpu.Architecture must be specified")
	}
	if len(scenario.Os.Name) == 0 {
		return "", fmt.Errorf("Os.Name must be specified")
	}
	uploaderBlessings, _ := security.RemoteBlessingNames(ctx, call.Security())
	// If there are multiple blessing names pack them into a single
	// string using NoExtension - which cannot be a substring in
	// any of the blessing names.
	uploader := strings.Join(uploaderBlessings, string(security.NoExtension))
	if err := s.store.Save(ctx, scenario, code, uploader, time.Now(), runs); err != nil {
		return "", err
	}
	// Construct a query that will retrieve these results.
	q := Query{
		CPU:      scenario.Cpu.Description,
		OS:       scenario.Os.Version,
		Uploader: uploader,
	}
	return fmt.Sprintf("%s%s", s.url, url.QueryEscape(q.String())), nil
}

type authorizer struct{}

func (authorizer) Authorize(ctx *context.T, call security.Call) error {
	got, rejected := security.RemoteBlessingNames(ctx, call)
	if len(got) > 0 {
		return nil
	}
	return verror.ErrNoAccess.Errorf(ctx, "access denied: %v", fmt.Errorf("refuse to store data for clients with no recognizable names (rejected names: %v)", rejected))
}

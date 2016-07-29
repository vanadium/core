// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package debuglib implements debug server support.
package debuglib

import (
	"strings"
	"time"

	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/x/ref/internal/logger"
	"v.io/x/ref/services/internal/httplib"
	"v.io/x/ref/services/internal/logreaderlib"
	"v.io/x/ref/services/internal/pproflib"
	"v.io/x/ref/services/internal/statslib"
	"v.io/x/ref/services/internal/vtracelib"
)

// dispatcher holds the state of the debug dispatcher.
type dispatcher struct {
	auth security.Authorizer
}

var _ rpc.Dispatcher = (*dispatcher)(nil)

func NewDispatcher(authorizer security.Authorizer) rpc.Dispatcher {
	return &dispatcher{authorizer}
}

// The first part of the names of the objects served by this dispatcher.
var rootName = "__debug"

func (d *dispatcher) Lookup(ctx *context.T, suffix string) (interface{}, security.Authorizer, error) {
	if suffix == "" {
		return rpc.ChildrenGlobberInvoker(rootName), d.auth, nil
	}
	if !strings.HasPrefix(suffix, rootName) {
		return nil, nil, nil
	}
	suffix = strings.TrimPrefix(suffix, rootName)
	suffix = strings.TrimLeft(suffix, "/")

	if suffix == "" {
		return rpc.ChildrenGlobberInvoker("logs", "pprof", "stats", "vtrace", "http"), d.auth, nil
	}
	parts := strings.SplitN(suffix, "/", 2)
	if len(parts) == 2 {
		suffix = parts[1]
	} else {
		suffix = ""
	}
	switch parts[0] {
	case "logs":
		return logreaderlib.NewLogFileService(logger.Manager(ctx).LogDir(), suffix), d.auth, nil
	case "pprof":
		return pproflib.NewPProfService(), d.auth, nil
	case "stats":
		return statslib.NewStatsService(suffix, 10*time.Second), d.auth, nil
	case "vtrace":
		return vtracelib.NewVtraceService(), d.auth, nil
	case "http":
		return httplib.NewHttpService(), d.auth, nil
	}
	return nil, d.auth, nil
}

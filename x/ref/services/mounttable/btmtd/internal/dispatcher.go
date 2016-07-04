// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"path"

	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	v23mt "v.io/v23/services/mounttable"

	"v.io/x/ref/lib/timekeeper"
)

type Config struct {
	GlobalAcl         access.AccessList
	MaxNodesPerUser   int64
	MaxServersPerUser int64
}

var clock = timekeeper.RealTime()

// For testing only.
func SetClock(c timekeeper.TimeKeeper) {
	clock = c
}

func NewDispatcher(bt *BigTable, config *Config) rpc.Dispatcher {
	return &dispatcher{bt, config}
}

type dispatcher struct {
	bt     *BigTable
	config *Config
}

func (d *dispatcher) Lookup(ctx *context.T, suffix string) (interface{}, security.Authorizer, error) {
	if suffix != "" {
		suffix = path.Clean(suffix)
	}
	return v23mt.MountTableServer(&mounttable{
		suffix: suffix,
		config: d.config,
		bt:     d.bt,
	}), security.AllowEveryone(), nil
}

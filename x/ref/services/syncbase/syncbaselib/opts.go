// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbaselib

import (
	"flag"
)

type Opts struct {
	Name            string
	RootDir         string
	Engine          string
	SkipPublishInNh bool
	DevMode         bool
	CpuProfile      string
	InitialDB       string
}

// Note: Where possible, we have flag default values be zero values, so that
// struct-based configuration matches flag-based configuration.
func (o *Opts) InitFlags(f *flag.FlagSet) {
	f.StringVar(&o.Name, "name", "", "Name to mount at.")
	f.StringVar(&o.RootDir, "root-dir", "", "Root dir for data storage. If empty, we write to a fresh directory created using ioutil.TempDir.")
	f.StringVar(&o.Engine, "engine", "", "Storage engine to use: memstore or leveldb. If empty, we use the default storage engine, currently leveldb.")
	f.BoolVar(&o.SkipPublishInNh, "skip-publish-in-nh", false, "Whether to skip publishing in the neighborhood.")
	f.BoolVar(&o.DevMode, "dev", false, "Whether to run in development mode; required for RPCs such as Service.DevModeUpdateVClock.")
	f.StringVar(&o.CpuProfile, "cpuprofile", "", "If specified, write the cpu profile to the given filename.")
	f.StringVar(&o.InitialDB, "initial-db", "", "If specified, a new database with the given id is created when setting up a brand new storage instance. Permissions for the database will be the service permissions; additionally, the blessing specified in the database id will have Read, Write, and Resolve. Format must conform to v.io/services/syncbase.Id.String: blessing,name")
}

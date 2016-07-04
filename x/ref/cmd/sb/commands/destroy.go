// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package commands

import (
	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase"
	"v.io/x/lib/cmdline"
)

var cmdDestroy = &cmdline.Command{
	Name:     "destroy",
	Short:    "Destroy a specified collection",
	Long:     "Destroy a specified collection.",
	ArgsName: "<collection_blessing>,<collection_name>",
	Runner:   SbRunner(runDestroy),
}

func runDestroy(ctx *context.T, db syncbase.Database, env *cmdline.Env, args []string) error {
	if got := len(args); got != 1 {
		return env.UsageErrorf("destroy: expected 1 arg, got %d", got)
	}

	if id, err := wire.ParseId(args[0]); err != nil {
		return err
	} else {
		return db.CollectionForId(id).Destroy(ctx)
	}
}

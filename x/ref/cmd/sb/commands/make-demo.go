// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package commands

import (
	"fmt"

	"v.io/v23/context"
	"v.io/v23/syncbase"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/cmd/sb/internal/demodb"
)

var cmdMakeDemo = &cmdline.Command{
	Name:   "make-demo",
	Short:  "Populate the db with dummy data",
	Long:   `Populate the db with dummy data.`,
	Runner: SbRunner(makeDemoDB),
}

func makeDemoDB(ctx *context.T, db syncbase.Database, env *cmdline.Env, _ []string) error {
	w := env.Stdout
	if err := demodb.PopulateDemoDB(ctx, db); err != nil {
		return fmt.Errorf("failed making demo collections: %v", err)
	}
	fmt.Fprintln(w, "Demo collections created and populated.")
	return nil
}

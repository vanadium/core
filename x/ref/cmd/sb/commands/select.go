// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package commands

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"v.io/v23/context"
	"v.io/v23/syncbase"
	"v.io/x/lib/cmdline"
)

var cmdSelect = &cmdline.Command{
	// TODO(zsterling): Wrap select in exec and add more general queries/deletes.
	Name:  "select",
	Short: "Display particular rows, or parts of rows",
	Long: `
Display particular rows, or parts of rows.
`,
	ArgsName: "<field_1,field_2,...> from <collection> [where <field> = <value>]",
	ArgsLong: `
<field_n> specifies a field in the collection <collection>.
`,
	Runner: SbRunner(selectFunc),
}

func selectFunc(ctx *context.T, db syncbase.Database, env *cmdline.Env, args []string) error {
	return queryExec(ctx, env.Stdout, db, "select "+strings.Join(args, " "))
}

// Split an error message into an offset and the remaining (i.e., rhs of offset) message.
// The convention for syncql is "<module><optional-rpc>[offset]<remaining-message>".
func splitError(err error) (int64, string) {
	errMsg := err.Error()
	idx1 := strings.Index(errMsg, "[")
	idx2 := strings.Index(errMsg, "]")
	if idx1 == -1 || idx2 == -1 {
		return 0, errMsg
	}
	offsetString := errMsg[idx1+1 : idx2]
	offset, err := strconv.ParseInt(offsetString, 10, 64)
	if err != nil {
		return 0, errMsg
	}
	return offset, errMsg[idx2+1:]
}

func queryExec(ctx *context.T, w io.Writer, d syncbase.Database, q string) error {
	columnNames, rs, err := d.Exec(ctx, q)
	if err != nil {
		off, msg := splitError(err)
		return fmt.Errorf("\n%s\n%s^\n%d: %s", q, strings.Repeat(" ", int(off)), off+1, msg)
	}
	return flagFormat.NewWriter(w).Write(columnNames, rs)
}

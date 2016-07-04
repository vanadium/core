// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package commands

import (
	"flag"
	"fmt"
	"io"

	"v.io/x/ref/cmd/sb/internal/writer"
)

type formatFlag string

func (f *formatFlag) Set(s string) error {
	for _, v := range []string{"table", "csv", "json"} {
		if s == v {
			*f = formatFlag(s)
			return nil
		}
	}
	return fmt.Errorf("unsupported -format %q", s)
}

func (f *formatFlag) String() string {
	return string(*f)
}

func (f formatFlag) NewWriter(w io.Writer) writer.FormattingWriter {
	switch f {
	case "table":
		return writer.NewTableWriter(w)
	case "csv":
		return writer.NewCSVWriter(w, flagCSVDelimiter)
	case "json":
		return writer.NewJSONWriter(w)
	default:
		panic("unexpected format:" + f)
	}
	return nil
}

var (
	flagFormat       formatFlag = "table"
	flagCSVDelimiter string
)

func init() {
	flag.Var(&flagFormat, "format", "Output format. 'table': human-readable table; 'csv': comma-separated values, use -csv-delimiter to control the delimiter; 'json': JSON objects.")
	flag.StringVar(&flagCSVDelimiter, "csv-delimiter", ",", "The delimiter used for the csv output format.")
}

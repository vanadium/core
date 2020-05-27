// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testutil

import (
	"io"
	"sort"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
)

// GlobName calls __Glob on the given object with the given pattern and returns
// a sorted list of matching object names, or an error.
func GlobName(ctx *context.T, name, pattern string) ([]string, []naming.GlobError, error) {
	client := v23.GetClient(ctx)
	call, err := client.StartCall(ctx, name, rpc.GlobMethod, []interface{}{pattern})
	if err != nil {
		return nil, nil, err
	}
	results := []string{}
	globErrors := []naming.GlobError{}
Loop:
	for {
		var gr naming.GlobReply
		switch err := call.Recv(&gr); err {
		case nil:
			switch v := gr.(type) {
			case naming.GlobReplyEntry:
				results = append(results, v.Value.Name)
			case naming.GlobReplyError:
				globErrors = append(globErrors, v.Value)
			}
		case io.EOF:
			break Loop
		default:
			return nil, nil, err
		}
	}
	sort.Strings(results)
	if err := call.Finish(); err != nil {
		return nil, nil, err
	}
	return results, globErrors, nil
}

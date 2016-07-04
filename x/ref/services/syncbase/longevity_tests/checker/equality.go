// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package checker

import (
	"fmt"

	"v.io/v23/context"
	"v.io/v23/syncbase"
	"v.io/x/ref/services/syncbase/longevity_tests/model"
	"v.io/x/ref/services/syncbase/longevity_tests/util"
)

// Equality Checker checks that all services in the model contain identical
// databases, collections, and rows.
type Equality struct{}

var _ Checker = (*Equality)(nil)

func (eq *Equality) Run(ctx *context.T, u model.Universe) error {
	// Collect all dump streams for each service in universe.
	dumpStreams := []*util.DumpStream{}
	for _, user := range u.Users {
		for _, device := range user.Devices {
			service := syncbase.NewService(device.Name)
			stream, err := util.NewDumpStream(ctx, service)
			if err != nil {
				return err
			}
			defer stream.Cancel()
			dumpStreams = append(dumpStreams, stream)
		}
	}

	// Check that all collections are identical and have identical rows.
	return equalDumpStreams(dumpStreams)
}

// equalRowStreams checks that multiple row streams contain identical rows.
func equalDumpStreams(streams []*util.DumpStream) error {
	firstStream := streams[0]
	restStreams := streams[1:]
	rowCount := 0

	for {
		// Advance the first stream.
		firstAdvance := firstStream.Advance()
		// Advance all other streams and make sure value agrees.
		for _, stream := range restStreams {
			gotAdvance := stream.Advance()
			if firstAdvance != gotAdvance {
				return fmt.Errorf("stream.Advance() was %v for service %q, but was %v for service %q",
					firstAdvance, firstStream.ServiceName, gotAdvance, stream.ServiceName)
			}
		}

		// Check all streams for errors.
		for _, stream := range streams {
			if err := stream.Err(); err != nil {
				return fmt.Errorf("stream.Err() was non-nil for service %q : %v", stream.ServiceName, stream.Err())
			}
		}

		// Stop if all Advances returned false.
		if !firstAdvance {
			break
		}

		// Get row from first stream.
		firstRow := firstStream.Row()
		// Check equality with rows from other streams.
		for _, stream := range restStreams {
			gotRow := stream.Row()
			if !firstRow.Equals(gotRow) {
				return fmt.Errorf("stream for service %q had row %v, but stream for service %q had row %v",
					firstStream.ServiceName, firstRow, stream.ServiceName, gotRow)
			}
		}

		rowCount++
	}
	fmt.Printf("checker found %d identical rows in %d databases\n", rowCount, len(streams))
	return nil
}

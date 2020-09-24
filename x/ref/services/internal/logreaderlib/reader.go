// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package logreaderlib

import (
	"bytes"
	"io"
	"strings"
	"time"

	"v.io/v23/context"
	"v.io/v23/verror"
)

// followReader implements the functionality of io.Reader, plus:
// - it can block for new input when the end of the file is reached, and
// - it aborts when the parent RPC is canceled.
type followReader struct {
	reader io.Reader
	ctx    *context.T
	offset int64
	follow bool
	buf    []byte
}

// newFollowReader is the factory for followReader.
//
// It assumes that the reader has already been advanced to startpos.
func newFollowReader(ctx *context.T, reader io.Reader, startpos int64, follow bool) *followReader {
	return &followReader{
		reader: reader,
		ctx:    ctx,
		offset: startpos,
		follow: follow,
	}
}

// tell returns the offset where the next read will start.
func (f *followReader) tell() int64 {
	return f.offset
}

func (f *followReader) read(b []byte) (int, error) {
	for {
		if f.ctx != nil {
			select {
			case <-f.ctx.Done():
				return 0, verror.ErrCanceled.Errorf(f.ctx, "canceled: %v", f.ctx.Err())
			default:
			}
		}
		n, err := f.reader.Read(b)
		if n == 0 && err == nil {
			// According to http://golang.org/pkg/io/#Reader, this
			// weird case should be treated as a no-op.
			continue
		}
		if n > 0 && err == io.EOF {
			err = nil
		}
		if err == io.EOF && f.follow {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		return n, err
	}
}

// readLine returns a whole line as a string, and the offset where it starts in
// the file. White spaces are removed from the beginning and the end of the line.
// If readLine returns an error, the other two return values should be discarded.
func (f *followReader) readLine() (string, int64, error) {
	startOff := f.offset
	var off int
	for {
		off = bytes.IndexByte(f.buf, '\n') + 1
		if off != 0 {
			break
		}
		b := make([]byte, 2048)
		n, err := f.read(b)
		if n > 0 {
			f.buf = append(f.buf, b[:n]...)
			continue
		}
		return "", 0, err
	}
	line := f.buf[:off-1] // -1 to remove the trailing \n
	f.buf = f.buf[off:]
	f.offset += int64(off)
	return strings.TrimSpace(string(line)), startOff, nil
}

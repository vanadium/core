// Copyright 2018 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"bytes"
	"errors"
	"io"

	"v.io/v23/vdl"
)

var (
	errEncodedBytesNotSupported = errors.New("pre-encoded bytes are not supported by the underlying vom encoder type")
)

// AlreadyEncodedBytes represents an already encoded vom byte-slice. It is intended
// for use when merging the output of multiple vom, independent, encoders.
type AlreadyEncodedBytes struct {
	Data []byte
}

// VDLWrite implements vdl.Writer.
func (eb *AlreadyEncodedBytes) VDLWrite(enc vdl.Encoder) error {
	uenc, ok := enc.(*encoder81)
	if !ok {
		return errEncodedBytesNotSupported
	}
	rd := bytes.NewBuffer(eb.Data)
	dec := NewDecoder(rd)
	for {
		var any vdl.Value
		if err := dec.Decode(&any); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if err := vdl.Write(uenc, any); err != nil {
			return err
		}
	}
	return nil
}

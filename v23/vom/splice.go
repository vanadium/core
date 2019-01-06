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
	errEncodedBytesNotSupported = errors.New("pre-encoded bytes are not supported by the underlying vom encoder/decoder type")
)

// MergeEncodedBytes represents an already encoded vom stream that is intended
// for use when merging the output of multiple, independent, vom encoders.
type MergeEncodedBytes struct {
	Data []byte
}

// VDLWrite implements vdl.Writer.
// Note that it decodes and re-encodes the vom stream and is consequently
// expensive.
func (eb *MergeEncodedBytes) VDLWrite(enc vdl.Encoder) error {
	e, ok := enc.(*encoder81)
	if !ok {
		return errEncodedBytesNotSupported
	}
	dec := NewDecoder(bytes.NewBuffer(eb.Data))
	for {
		var any vdl.Value
		if err := dec.Decode(&any); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if err := vdl.Write(e, any); err != nil {
			return err
		}
	}
	return nil
}

// ExtractEncodedBytes represents an already encoded stream that has been
// extracted from a separate, independent vom stream. NumToDecode controls
// how many values are to be read from the existing stream, or 0 to read
// all values.
type ExtractEncodedBytes struct {
	Data                 []byte
	NumToDecode, Decoded int
}

// VDLRead implements vdl.Reader.
func (eb *ExtractEncodedBytes) VDLRead(dec vdl.Decoder) error {
	d, ok := dec.(*decoder81)
	if !ok {
		return errEncodedBytesNotSupported
	}
	buf := &bytes.Buffer{}
	enc := NewEncoder(buf)
	n := 0
	for {
		if eb.NumToDecode > 0 && n == eb.NumToDecode {
			break
		}
		var any vdl.Value
		if err := vdl.Read(d, &any); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if err := enc.Encode(any); err != nil {
			return err
		}
		n++
	}
	eb.Data = buf.Bytes()
	eb.Decoded = n
	return nil
}

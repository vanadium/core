// Copyright 2018 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

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
func (eb *MergeEncodedBytes) VDLWrite(enc vdl.Encoder) error {
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
	fmt.Fprintf(os.Stderr, "EXTRACT VDL.READ\n\n")
	udec, ok := dec.(*decoder81)
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
		if err := vdl.Read(udec, &any); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if err := enc.Encode(any); err != nil {
			fmt.Fprintf(os.Stderr, "ERR: %v\n", err)
			return err
		}
		n++
	}
	eb.Data = buf.Bytes()
	eb.Decoded = n
	return nil
}

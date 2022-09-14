// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package message

import (
	"v.io/v23/context"
)

// ReadData reads a Data message from the supplied buffer if one is present,
// returning true and the error for parsing the contents of the buffer. If
// the buffer does not contain a Data message then false is returned.
func ReadData(ctx *context.T, from []byte, to *Data) (bool, error) {
	if len(from) == 0 || from[0] != dataType {
		return false, nil
	}
	return true, to.read(ctx, from[1:])
}

// SetPlaintextDataPayload is used to associate a payload that was sent in
// the clear (following a message with the DisableEncryptionFlag flag set) with
// the immediately preceding received message. The 'nocopy' parameter indicates
// whether a subsequent call to CopyBuffers needs to copy the payload.
func SetPlaintextDataPayload(msg *Data, payload []byte, nocopy bool) {
	msg.Payload = [][]byte{payload}
	msg.nocopy = nocopy
}

// ReadSetup reads a Setup message from the supplied buffer if one is present,
// returning true and the error for parsing the contents of the buffer. If
// the buffer does not contain a Setup message then false is returned.
func ReadSetup(ctx *context.T, from []byte, to *Setup) (bool, error) {
	if len(from) == 0 || from[0] != setupType {
		return false, nil
	}
	return true, to.read(ctx, from[1:])
}

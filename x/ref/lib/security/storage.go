// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"io"

	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/x/ref/lib/security/serialization"
)

var (
	errBadDataOrSig = verror.Register(pkgPath+".errBadDataOrSig", verror.NoRetry, "{1:}{2:} invalid data/signature handles data:{3} sig:{4}{:_}")
)

func encodeAndStore(obj interface{}, data, signature io.WriteCloser, signer serialization.Signer) error {
	if data == nil || signature == nil {
		return verror.New(errBadDataOrSig, nil, data, signature)
	}
	swc, err := serialization.NewSigningWriteCloser(data, signature, signer, nil)
	if err != nil {
		return err
	}
	enc := vom.NewEncoder(swc)
	if err := enc.Encode(obj); err != nil {
		swc.Close()
		return err
	}
	return swc.Close()
}

func decodeFromStorage(obj interface{}, data, signature io.ReadCloser, publicKey security.PublicKey) error {
	if data == nil || signature == nil {
		return verror.New(errBadDataOrSig, nil, data, signature)
	}
	defer data.Close()
	defer signature.Close()
	vr, err := serialization.NewVerifyingReader(data, signature, publicKey)
	if err != nil {
		return err
	}
	dec := vom.NewDecoder(vr)
	return dec.Decode(obj)
}

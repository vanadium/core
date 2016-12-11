// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package serialization

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"

	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/v23/vom"
)

const pkgPath = "v.io/x/ref/lib/security/serialization"

var (
	errCantBeNilVerifier      = verror.Register(pkgPath+".errCantBeNilVerifier", verror.NoRetry, "{1:}{2:} data:{3} signature:{4} key:{5} cannot be nil{:_}")
	errModifiedSinceWritten   = verror.Register(pkgPath+".errModifiedSinceWritten", verror.NoRetry, "{1:}{2:} data has been modified since being written{:_}")
	errCantDecodeHeader       = verror.Register(pkgPath+".errCantDecodeHeader", verror.NoRetry, "{1:}{2:} failed to decode header{:_}")
	errCantVerifySig          = verror.Register(pkgPath+".errCantVerifySig", verror.NoRetry, "{1:}{2:} signature verification failed{:_}")
	errBadTypeFromSigReader   = verror.Register(pkgPath+".errBadTypeFromSigReader", verror.NoRetry, "{1:}{2:} invalid data of type: {3} read from signature Reader{:_}")
	errUnexpectedDataAfterSig = verror.Register(pkgPath+".errUnexpectedDataAfterSig", verror.NoRetry, "{1:}{2:} unexpected data found after signature{:_}")
)

// verifyingReader implements io.Reader.
type verifyingReader struct {
	data io.Reader

	chunkSizeBytes int64
	curChunk       bytes.Buffer
	hashes         bytes.Buffer
}

func (r *verifyingReader) Read(p []byte) (int, error) {
	bytesRead := 0
	for len(p) > 0 {
		if err := r.readChunk(); err != nil {
			return bytesRead, err
		}

		n, err := r.curChunk.Read(p)
		bytesRead = bytesRead + n
		if err != nil {
			return bytesRead, err
		}

		p = p[n:]
	}
	return bytesRead, nil
}

// NewVerifyingReader returns an io.Reader that ensures that all data returned
// by Read calls was written using a NewSigningWriter (by a principal possessing
// a signer corresponding to the provided public key), and has not been modified
// since (ensuring integrity and authenticity of data).
func NewVerifyingReader(data, signature io.Reader, key security.PublicKey) (io.Reader, error) {
	if (data == nil) || (signature == nil) || (key == nil) {
		return nil, verror.New(errCantBeNilVerifier, nil, data, signature, key)
	}
	r := &verifyingReader{data: data}
	if err := r.verifySignature(signature, key); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *verifyingReader) readChunk() error {
	if r.curChunk.Len() > 0 {
		return nil
	}
	hash := make([]byte, sha256.Size)
	if _, err := r.hashes.Read(hash); err == io.EOF {
		return nil
	} else if err != nil {
		return err
	}

	if _, err := io.CopyN(&r.curChunk, r.data, int64(r.chunkSizeBytes)); err != nil && err != io.EOF {
		return err
	}

	if wantHash := sha256.Sum256(r.curChunk.Bytes()); !bytes.Equal(hash, wantHash[:]) {
		return verror.New(errModifiedSinceWritten, nil)
	}
	return nil
}

func (r *verifyingReader) verifySignature(signature io.Reader, key security.PublicKey) error {
	signatureHash := sha256.New()
	dec := vom.NewDecoder(signature)
	var h SignedHeader
	if err := dec.Decode(&h); err != nil {
		return verror.New(errCantDecodeHeader, nil, err)
	}
	r.chunkSizeBytes = h.ChunkSizeBytes
	if err := binary.Write(signatureHash, binary.LittleEndian, r.chunkSizeBytes); err != nil {
		return err
	}

	var signatureFound bool
	for !signatureFound {
		var i SignedData
		if err := dec.Decode(&i); err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		switch v := i.(type) {
		case SignedDataHash:
			if _, err := io.MultiWriter(&r.hashes, signatureHash).Write(v.Value[:]); err != nil {
				return err
			}
		case SignedDataSignature:
			signatureFound = true
			if !v.Value.Verify(key, signatureHash.Sum(nil)) {
				return verror.New(errCantVerifySig, nil)
			}
		default:
			return verror.New(errBadTypeFromSigReader, nil, fmt.Sprintf("%T", i))
		}
	}
	// Verify that no more data can be read from the signature Reader.
	if _, err := signature.Read(make([]byte, 1)); err != io.EOF {
		return verror.New(errUnexpectedDataAfterSig, nil)
	}
	return nil
}

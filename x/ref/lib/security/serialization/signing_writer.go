// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package serialization

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash"
	"io"

	"v.io/v23/security"
	"v.io/v23/vom"
)

const defaultChunkSizeBytes = 1 << 20

// signingWriter implements io.WriteCloser.
type signingWriter struct {
	data      io.WriteCloser
	signature io.WriteCloser
	signer    Signer

	chunkSizeBytes int64
	curChunk       bytes.Buffer
	signatureHash  hash.Hash
	sigEnc         *vom.Encoder
}

func (w *signingWriter) Write(p []byte) (int, error) {
	bytesWritten := 0
	for len(p) > 0 {
		pLimited := p
		curChunkFreeBytes := w.chunkSizeBytes - int64(w.curChunk.Len())
		if int64(len(pLimited)) > curChunkFreeBytes {
			pLimited = pLimited[:curChunkFreeBytes]
		}

		n, err := w.curChunk.Write(pLimited)
		bytesWritten += n
		if err != nil {
			return bytesWritten, err
		}
		p = p[n:]

		if err := w.commitChunk(false); err != nil {
			return bytesWritten, err
		}
	}
	return bytesWritten, nil
}

func (w *signingWriter) Close() error {
	if w.curChunk.Len() > 0 {
		if err := w.commitChunk(true); err != nil {
			defer w.close()
			return err
		}
	}
	if err := w.commitSignature(); err != nil {
		defer w.close()
		return err
	}
	return w.close()
}

// Options specifies parameters to tune a SigningWriteCloser.
type Options struct {
	// ChunkSizeBytes controls the maximum amount of memory devoted to buffering
	// data provided to Write calls. See NewSigningWriteCloser.
	ChunkSizeBytes int64
}

// Signer is the interface for digital signature operations used by NewSigningWriteCloser.
type Signer interface {
	Sign(message []byte) (security.Signature, error)
	PublicKey() security.PublicKey
}

// NewSigningWriteCloser returns an io.WriteCloser that writes data along
// with an appropriate signature that establishes the integrity and
// authenticity of the data. It behaves as follows:
//     * A Write call writes chunks (of size provided by the Options or
//       1MB by default) of data to the provided data WriteCloser and a
//       hash of the chunks to the provided signature WriteCloser.
//     * A Close call writes a signature (computed using the provided
//       signer) of all the hashes written, and then closes the data and
//       signature WriteClosers.
func NewSigningWriteCloser(data, signature io.WriteCloser, s Signer, opts *Options) (io.WriteCloser, error) {
	if (data == nil) || (signature == nil) || (s == nil) {
		return nil, fmt.Errorf("data:%v signature:%v signer:%v cannot be nil", data, signature, s)
	}
	enc := vom.NewEncoder(signature)
	w := &signingWriter{data: data, signature: signature, signer: s, signatureHash: sha256.New(), chunkSizeBytes: defaultChunkSizeBytes, sigEnc: enc}

	if opts != nil {
		w.chunkSizeBytes = opts.ChunkSizeBytes
	}

	if err := w.commitHeader(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *signingWriter) commitHeader() error {
	if err := binary.Write(w.signatureHash, binary.LittleEndian, w.chunkSizeBytes); err != nil {
		return err
	}
	if err := w.sigEnc.Encode(SignedHeader{w.chunkSizeBytes}); err != nil {
		return err
	}
	return nil
}

func (w *signingWriter) commitChunk(force bool) error {
	if !force && int64(w.curChunk.Len()) < w.chunkSizeBytes {
		return nil
	}

	hashBytes := sha256.Sum256(w.curChunk.Bytes())
	if _, err := io.CopyN(w.data, &w.curChunk, int64(w.curChunk.Len())); err != nil {
		return err
	}
	if _, err := w.signatureHash.Write(hashBytes[:]); err != nil {
		return err
	}
	return w.sigEnc.Encode(SignedDataHash{hashBytes})
}

func (w *signingWriter) commitSignature() error {
	sig, err := w.signer.Sign(w.signatureHash.Sum(nil))
	if err != nil {
		return fmt.Errorf("signing failed: %v", err)
	}

	return w.sigEnc.Encode(SignedDataSignature{sig})
}

func (w *signingWriter) close() error {
	var closeErr error
	if err := w.data.Close(); err != nil && closeErr == nil {
		closeErr = err
	}
	if err := w.signature.Close(); err != nil && closeErr == nil {
		closeErr = err
	}
	return closeErr
}

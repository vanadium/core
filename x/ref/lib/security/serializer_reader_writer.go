// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"io"
	"os"

	"v.io/v23/security"
)

type serializationSigner struct {
	s security.Signer
}

func (s *serializationSigner) Sign(message []byte) (security.Signature, error) {
	return s.s.Sign([]byte(security.SignatureForMessageSigning), message)
}

func (s *serializationSigner) PublicKey() security.PublicKey {
	return s.s.PublicKey()
}

// SerializerWriter is a factory for managing the readers used for
// deserialization of signed data.
type SerializerReader interface {
	// Readers returns io.ReadCloser for reading serialized data and its
	// integrity signature.
	Readers() (data io.ReadCloser, signature io.ReadCloser, err error)
}

// SerializerWriter is a factory for managing the writers used for
// serialization of signed data.
type SerializerWriter interface {
	// Writers returns io.WriteCloser for writing serialized data and its
	// integrity signature.
	Writers() (data io.WriteCloser, signature io.WriteCloser, err error)
}

// fileSerializer implements SerializerReaderWriter that persists state to files.
type fileSerializer struct {
	dataFilePath      string
	signatureFilePath string
}

// newFileSerializer creates a FileSerializer with the given data and signature files.
func newFileSerializer(dataFilePath, signatureFilePath string) *fileSerializer {
	return &fileSerializer{
		dataFilePath:      dataFilePath,
		signatureFilePath: signatureFilePath,
	}
}

func (fs *fileSerializer) Readers() (io.ReadCloser, io.ReadCloser, error) {
	data, err := os.Open(fs.dataFilePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, err
	}
	signature, err := os.Open(fs.signatureFilePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, err
	}
	if data == nil || signature == nil {
		return nil, nil, nil
	}
	return data, signature, nil
}

func (fs *fileSerializer) Writers() (io.WriteCloser, io.WriteCloser, error) {
	// Remove previous version of the files
	// TODO(cnicolaou): use tmp files and os.Rename, which is awkward
	//   given the current design since the rename needs to happen on
	//   the file close.
	os.Remove(fs.dataFilePath)
	os.Remove(fs.signatureFilePath)
	data, err := os.Create(fs.dataFilePath)
	if err != nil {
		return nil, nil, err
	}
	signature, err := os.Create(fs.signatureFilePath)
	if err != nil {
		return nil, nil, err
	}
	return data, signature, nil
}

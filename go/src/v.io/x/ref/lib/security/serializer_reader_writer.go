// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"io"
	"os"
)

// SerializerReaderWriter is a factory for managing the readers and writers used for
// serialization and deserialization of signed data.
type SerializerReaderWriter interface {
	// Readers returns io.ReadCloser for reading serialized data and its
	// integrity signature.
	Readers() (data io.ReadCloser, signature io.ReadCloser, err error)
	// Writers returns io.WriteCloser for writing serialized data and its
	// integrity signature.
	Writers() (data io.WriteCloser, signature io.WriteCloser, err error)
}

// FileSerializer implements SerializerReaderWriter that persists state to files.
type FileSerializer struct {
	dataFilePath      string
	signatureFilePath string
}

// NewFileSerializer creates a FileSerializer with the given data and signature files.
func NewFileSerializer(dataFilePath, signatureFilePath string) *FileSerializer {
	return &FileSerializer{
		dataFilePath:      dataFilePath,
		signatureFilePath: signatureFilePath,
	}
}

func (fs *FileSerializer) Readers() (io.ReadCloser, io.ReadCloser, error) {
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

func (fs *FileSerializer) Writers() (io.WriteCloser, io.WriteCloser, error) {
	// Remove previous version of the files
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

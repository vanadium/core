// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"io"
	"os"

	"cloudeng.io/os/lockedfile"
)

// SerializerReaderWriter is a factory for managing the readers and writers used for
// serialization and deserialization of signed data. Implementations must
// provide appropriate inter and intra-process synchronization.
type SerializerReaderWriter interface {
	// Readers returns io.ReadCloser for reading serialized data and its
	// integrity signature. The returned unlock function must be called after
	// the data and signature Closers are called and regardless of whether
	// Readers returns an error.
	Readers() (data io.ReadCloser, signature io.ReadCloser, unlock func(), err error)
	// Writers returns io.WriteCloser for writing serialized data and its
	// integrity signature. The returned unlock function must be called after
	// the data and signature Closers are called.
	Writers() (data io.WriteCloser, signature io.WriteCloser, unlock func(), err error)

	String() string
}

// FileSerializer implements SerializerReaderWriter that persists state to files.
type FileSerializer struct {
	fileMutex         *lockedfile.Mutex
	lockFilePath      string
	dataFilePath      string
	signatureFilePath string
}

// NewFileSerializer creates a FileSerializer with the given data and signature files.
// A lockfile is used to synchronise access to the data and signature
// files. The lock is acquired when when Readers/Writers are called
// which returns the unlock function.
func NewFileSerializer(lockFilePath, dataFilePath, signatureFilePath string) *FileSerializer {
	return &FileSerializer{
		fileMutex:         lockedfile.MutexAt(lockFilePath),
		dataFilePath:      dataFilePath,
		signatureFilePath: signatureFilePath,
	}
}

func (fs *FileSerializer) String() string {
	return fs.dataFilePath
}

func (fs *FileSerializer) Readers() (io.ReadCloser, io.ReadCloser, func(), error) {
	unlock, err := fs.fileMutex.Lock()
	if err != nil {
		return nil, nil, nil, err
	}
	data, err := os.Open(fs.dataFilePath)
	if err != nil && !os.IsNotExist(err) {
		unlock()
		return nil, nil, nil, err
	}
	signature, err := os.Open(fs.signatureFilePath)
	if err != nil && !os.IsNotExist(err) {
		unlock()
		return nil, nil, nil, err
	}
	if data == nil || signature == nil {
		unlock()
		return nil, nil, func() {}, nil
	}
	return data, signature, unlock, nil
}

func (fs *FileSerializer) Writers() (io.WriteCloser, io.WriteCloser, func(), error) {
	unlock, err := fs.fileMutex.Lock()
	if err != nil {
		return nil, nil, nil, err
	}
	// Remove previous version of the files
	os.Remove(fs.dataFilePath)
	os.Remove(fs.signatureFilePath)
	data, err := os.Create(fs.dataFilePath)
	if err != nil {
		unlock()
		return nil, nil, nil, err
	}
	signature, err := os.Create(fs.signatureFilePath)
	if err != nil {
		unlock()
		return nil, nil, nil, err
	}
	return data, signature, unlock, nil
}

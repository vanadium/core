// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package multipart implements an http.File that acts as one logical file
// backed by several physical files (the 'parts').
package multipart

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

var errInternal = fmt.Errorf("internal error")

// NewFile creates the multipart file out of the provided parts.
// The sizes of the parts are captured at the outset and not updated
// for the lifetime of the multipart file (any subsequent modifications
// in the parts will cause Read and Seek to work incorrectly).
func NewFile(name string, parts []*os.File) (http.File, error) {
	fileParts := make([]filePart, len(parts))
	for i, p := range parts {
		stat, err := p.Stat()
		if err != nil {
			return nil, err
		}
		size := stat.Size()
		// TODO(caprita): we can relax this restriction later.
		if size == 0 {
			return nil, fmt.Errorf("Part is empty")
		}
		fileParts[i] = filePart{file: p, size: size}
	}
	return &multipartFile{name: name, parts: fileParts}, nil
}

type filePart struct {
	file *os.File
	size int64
}

type multipartFile struct {
	name       string
	parts      []filePart
	activePart int
	partOffset int64
}

func (m *multipartFile) currPos() (res int64) {
	for i := 0; i < m.activePart; i++ {
		res += m.parts[i].size
	}
	res += m.partOffset
	return
}

func (m *multipartFile) totalSize() (res int64) {
	for _, p := range m.parts {
		res += p.size
	}
	return
}

// Readdir is not implemented.
func (*multipartFile) Readdir(int) ([]os.FileInfo, error) {
	return nil, fmt.Errorf("Not implemented")
}

type fileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

// Name returns the name of the multipart file.
func (f *fileInfo) Name() string {
	return f.name
}

// Size returns the size of the multipart file (the sum of all parts).
func (f *fileInfo) Size() int64 {
	return f.size
}

// Mode is currently hardcoded to 0700.
func (f *fileInfo) Mode() os.FileMode {
	return f.mode
}

// ModTime is set to the current time.
func (f *fileInfo) ModTime() time.Time {
	return f.modTime
}

// IsDir always returns false.
func (f *fileInfo) IsDir() bool {
	return false
}

// Sys always returns nil.
func (f *fileInfo) Sys() interface{} {
	return nil
}

// Stat describes the multipart file.
func (m *multipartFile) Stat() (os.FileInfo, error) {
	return &fileInfo{
		name:    m.name,
		size:    m.totalSize(),
		mode:    0700,
		modTime: time.Now(),
	}, nil
}

// Close closes all the parts.
func (m *multipartFile) Close() error {
	var lastErr error
	for _, p := range m.parts {
		if err := p.file.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// Read reads from the parts in sequence.
func (m *multipartFile) Read(buf []byte) (int, error) {
	if m.activePart >= len(m.parts) {
		return 0, io.EOF
	}
	p := m.parts[m.activePart]
	n, err := p.file.Read(buf)
	m.partOffset += int64(n)
	if m.partOffset > p.size {
		// Likely, the file has changed.
		return 0, errInternal
	}
	if m.partOffset == p.size {
		m.activePart++
		if m.activePart < len(m.parts) {
			if _, err := m.parts[m.activePart].file.Seek(0, 0); err != nil {
				return 0, err
			}
			m.partOffset = 0
		}
	}
	return n, err
}

// Seek seeks into the part corresponding to the global offset.
func (m *multipartFile) Seek(offset int64, whence int) (int64, error) {
	var target int64
	switch whence {
	case 0:
		target = offset
	case 1:
		target = m.currPos() + offset
	case 2:
		target = m.totalSize() - offset
	default:
		return 0, fmt.Errorf("invalid whence: %d", whence)
	}
	if target < 0 || target > m.totalSize() {
		return 0, fmt.Errorf("invalid offset")
	}
	var c int64
	for i, p := range m.parts {
		if pSize := p.size; c+pSize <= target {
			c += pSize
			continue
		}
		m.activePart = i
		if _, err := p.file.Seek(target-c, 0); err != nil {
			return 0, err
		}
		m.partOffset = target - c
		return target, nil
	}
	// target <= m.totalSize() should ensure this is never reached.
	return 0, errInternal // Should not be reached.
}

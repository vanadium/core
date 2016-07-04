// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"crypto/sha256"
	"encoding/base64"
	"io/ioutil"
	"os"
	"path/filepath"

	"v.io/v23/security"
	"v.io/v23/vom"
)

// NewFileStorage returns an AgentStore implementation that uses the local file
// system, with all files placed under the directory 'dir'.
func NewFileStorage(dir string) AgentStorage {
	return &fileStorage{dir}
}

type fileStorage struct {
	baseDir string
}

func (f *fileStorage) secretToFile(secret string) string {
	hash := sha256.Sum256([]byte(secret))
	name := base64.RawURLEncoding.EncodeToString(hash[:])
	return filepath.Join(f.baseDir, name)
}

func (f *fileStorage) Get(secret string) (blessings security.Blessings, err error) {
	var data []byte
	data, err = ioutil.ReadFile(f.secretToFile(secret))
	if err != nil {
		return
	}
	err = vom.Decode(data, &blessings)
	return
}

func (f *fileStorage) Put(secret string, blessings security.Blessings) error {
	file, err := os.OpenFile(f.secretToFile(secret), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if err := vom.NewEncoder(file).Encode(blessings); err != nil {
		return err
	}
	return file.Close()
}

func (f *fileStorage) Delete(secret string) error {
	if err := os.Remove(f.secretToFile(secret)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

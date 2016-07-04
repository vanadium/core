// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/x/lib/ibe"
	"v.io/x/ref/lib/security/bcrypter"
	"v.io/x/ref/services/iberoot"
)

const (
	keyFile    = "master-key"
	paramsFile = "master-params"
)

// SaveMaster saves the provided IBE Master object into the provided
// directory.
//
// The directory must not contain any previously saved Master objects.
// A new directory is created if it does not already exist.
func SaveMaster(master ibe.Master, dir string) error {
	if err := mkDir(dir); err != nil {
		return err
	}
	paramsBytes, err := ibe.MarshalParams(master.Params())
	if err != nil {
		return fmt.Errorf("failed to marshal master params: %v", err)
	}
	keyBytes, err := ibe.MarshalMasterKey(master)
	if err != nil {
		return fmt.Errorf("failed to marshal master private key: %v", err)
	}

	paramsFilePath := filepath.Join(dir, paramsFile)
	if err := writeFile(paramsFilePath, paramsBytes); err != nil {
		return fmt.Errorf("failed to write master params: %v", err)
	}
	if err := writeFile(filepath.Join(dir, keyFile), keyBytes); err != nil {
		defer os.Remove(paramsFilePath)
		return fmt.Errorf("failed to write master private key: %v", err)
	}
	return nil
}

type root struct {
	name string
	root *bcrypter.Root
}

func NewRootServer(keyDir, name string) (iberoot.RootServerStub, error) {
	params, err := loadParams(filepath.Join(keyDir, paramsFile))
	if err != nil {
		return nil, err
	}
	master, err := loadMaster(params, filepath.Join(keyDir, keyFile))
	if err != nil {
		return nil, err
	}
	return iberoot.RootServer(&root{name: name, root: bcrypter.NewRoot(name, master)}), nil
}

func (r *root) SeekPrivateKeys(ctx *context.T, call rpc.ServerCall) ([]bcrypter.WirePrivateKey, error) {
	remoteBlessings, rejected := security.RemoteBlessingNames(ctx, call.Security())
	blessings := filterBlessings(remoteBlessings, security.BlessingPattern(r.name))
	if len(blessings) == 0 {
		return nil, NewErrUnrecognizedRemoteBlessings(ctx, remoteBlessings, rejected, r.name)
	}
	ret := make([]bcrypter.WirePrivateKey, len(blessings))
	for i, b := range blessings {
		key, err := r.root.Extract(ctx, b)
		if err != nil {
			return nil, NewErrInternal(ctx, err)
		}
		if err := key.ToWire(&ret[i]); err != nil {
			return nil, NewErrInternal(ctx, err)
		}
	}
	return ret, nil
}

func (r *root) Params(ctx *context.T, call rpc.ServerCall) (bcrypter.WireParams, error) {
	params := r.root.Params()
	var ret bcrypter.WireParams
	if err := params.ToWire(&ret); err != nil {
		return bcrypter.WireParams{}, err
	}
	return ret, nil
}

func filterBlessings(blessings []string, pattern security.BlessingPattern) []string {
	var ret []string
	for _, b := range blessings {
		if pattern.MatchedBy(b) {
			ret = append(ret, b)
		}
	}
	return ret
}

func loadParams(filename string) (ibe.Params, error) {
	contents, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return ibe.UnmarshalParams(contents)
}

func loadMaster(params ibe.Params, filename string) (ibe.Master, error) {
	contents, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return ibe.UnmarshalMasterKey(params, contents)
}

func mkDir(dir string) error {
	if finfo, err := os.Stat(dir); err == nil {
		if !finfo.IsDir() {
			return fmt.Errorf("%v is not a directory", dir)
		}
	} else if os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed to create directory %v: %v", dir, err)
		}
	} else {
		return err
	}
	return nil
}

func writeFile(path string, data []byte) error {
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return fmt.Errorf("file %v already exists", path)
	}
	if err := ioutil.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write file %v: %v", path, err)
	}
	return nil
}

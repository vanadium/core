// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package impl

// Code to manage the persistence of which systemName is associated with
// a given application instance.

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"v.io/v23/verror"
	"v.io/x/ref/services/device/internal/errors"
)

func saveSystemNameForInstance(dir, systemName string) error {
	snp := filepath.Join(dir, "systemname")
	if err := ioutil.WriteFile(snp, []byte(systemName), 0600); err != nil {
		return verror.New(errors.ErrOperationFailed, nil, fmt.Sprintf("WriteFile(%v, %v) failed: %v", snp, systemName, err))
	}
	return nil
}

func readSystemNameForInstance(dir string) (string, error) {
	snp := filepath.Join(dir, "systemname")
	name, err := ioutil.ReadFile(snp)
	if err != nil {
		return "", verror.New(errors.ErrOperationFailed, nil, fmt.Sprintf("ReadFile(%v) failed: %v", snp, err))
	}
	return string(name), nil
}

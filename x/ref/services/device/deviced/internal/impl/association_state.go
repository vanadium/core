// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package impl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"v.io/v23/services/device"
	"v.io/v23/verror"
)

// BlessingSystemAssociationStore manages a persisted association between
// Vanadium blessings and system account names.
type BlessingSystemAssociationStore interface {
	// SystemAccountForBlessings returns a system name from the blessing to
	// system name association store if one exists for any of the listed
	// blessings.
	SystemAccountForBlessings(blessings []string) (string, bool)

	// AllBlessingSystemAssociations returns all of the current Blessing to system
	// account associations.
	AllBlessingSystemAssociations() ([]device.Association, error)

	// AssociateSystemAccountForBlessings associates the provided systenName with each
	// provided blessing.
	AssociateSystemAccountForBlessings(blessings []string, systemName string) error

	// DisassociateSystemAccountForBlessings removes associations for the provided blessings.
	DisassociateSystemAccountForBlessings(blessings []string) error
}

type association struct {
	data     map[string]string
	filename string
	sync.Mutex
}

func (u *association) SystemAccountForBlessings(blessings []string) (string, bool) {
	u.Lock()
	defer u.Unlock()

	systemName := ""
	present := false

	for _, n := range blessings {
		if systemName, present = u.data[n]; present {
			break
		}
	}
	return systemName, present
}

func (u *association) AllBlessingSystemAssociations() ([]device.Association, error) {
	u.Lock()
	defer u.Unlock()
	assocs := make([]device.Association, 0)

	for k, v := range u.data {
		assocs = append(assocs, device.Association{IdentityName: k, AccountName: v})
	}
	return assocs, nil
}

func (u *association) serialize() (err error) {
	f, err := os.OpenFile(u.filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return verror.New(verror.ErrNoExist, nil, "Could not open association file for writing", u.filename, err)
	}
	defer func() {
		if closerr := f.Close(); closerr != nil {
			err = closerr
		}
	}()

	enc := json.NewEncoder(f)
	return enc.Encode(u.data)
}

func (u *association) AssociateSystemAccountForBlessings(blessings []string, systemName string) error {
	u.Lock()
	defer u.Unlock()

	for _, n := range blessings {
		u.data[n] = systemName
	}
	return u.serialize()
}

func (u *association) DisassociateSystemAccountForBlessings(blessings []string) error {
	u.Lock()
	defer u.Unlock()

	for _, n := range blessings {
		delete(u.data, n)
	}
	return u.serialize()
}

func NewBlessingSystemAssociationStore(root string) (BlessingSystemAssociationStore, error) {
	nddir := filepath.Join(root, "device-manager", "device-data")
	if err := os.MkdirAll(nddir, os.FileMode(0700)); err != nil {
		return nil, verror.New(verror.ErrNoExist, nil, "Could not create device-data directory", nddir, err)
	}
	msf := filepath.Join(nddir, "associated.accounts")

	f, err := os.Open(msf)
	if err != nil && os.IsExist(err) {
		return nil, verror.New(verror.ErrNoExist, nil, "Could not open association file", msf, err)

	}
	defer f.Close()

	a := &association{filename: msf, data: make(map[string]string)}

	if err == nil {
		dec := json.NewDecoder(f)
		err := dec.Decode(&a.data)
		if err != nil {
			return nil, verror.New(verror.ErrNoExist, nil, "Could not read association file", msf, err)
		}
	}
	return BlessingSystemAssociationStore(a), nil
}

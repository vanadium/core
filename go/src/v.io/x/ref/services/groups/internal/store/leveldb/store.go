// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build leveldb

package leveldb

import (
	"strconv"

	"v.io/v23/context"
	"v.io/v23/vdl"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/x/ref/services/groups/internal/store"
	istore "v.io/x/ref/services/syncbase/store"
	"v.io/x/ref/services/syncbase/store/leveldb"
)

type entry struct {
	Value   interface{}
	Version uint64
}

type T struct {
	db istore.Store
}

var _ store.Store = (*T)(nil)

// Open opens a groups server store located at the given path,
// creating it if it doesn't exist.
func Open(path string) (store.Store, error) {
	var _ context.T
	db, err := leveldb.Open(path, leveldb.OpenOptions{CreateIfMissing: true, ErrorIfExists: false})
	if err != nil {
		return nil, convertError(err)
	}
	return &T{db: db}, nil
}

func (st *T) Get(key string, valbuf interface{}) (version string, err error) {
	e, err := get(st.db, key)
	if err != nil {
		return "", err
	}
	if err := vdl.Convert(valbuf, e.Value); err != nil {
		return "", convertError(err)
	}
	return strconv.FormatUint(e.Version, 10), nil
}

func (st *T) Insert(key string, value interface{}) error {
	return istore.RunInTransaction(st.db, func(tx istore.Transaction) error {
		if _, err := get(tx, key); verror.ErrorID(err) != store.ErrUnknownKey.ID {
			if err != nil {
				return err
			}
			return verror.New(store.ErrKeyExists, nil, key)
		}
		return put(tx, key, &entry{Value: value})
	})
}

func (st *T) Update(key string, value interface{}, version string) error {
	return istore.RunInTransaction(st.db, func(tx istore.Transaction) error {
		e, err := get(tx, key)
		if err != nil {
			return err
		}
		if err := e.checkVersion(version); err != nil {
			return err
		}
		return put(tx, key, &entry{Value: value, Version: e.Version + 1})
	})
}

func (st *T) Delete(key string, version string) error {
	return istore.RunInTransaction(st.db, func(tx istore.Transaction) error {
		e, err := get(tx, key)
		if err != nil {
			return err
		}
		if err := e.checkVersion(version); err != nil {
			return err
		}
		return delete(tx, key)
	})
}

func (st *T) Close() error {
	return convertError(st.db.Close())
}

func get(st istore.StoreReader, key string) (*entry, error) {
	bytes, _ := st.Get([]byte(key), nil)
	if bytes == nil {
		return nil, verror.New(store.ErrUnknownKey, nil, key)
	}
	e := &entry{}
	if err := vom.Decode(bytes, e); err != nil {
		return nil, convertError(err)
	}
	return e, nil
}

func put(stw istore.StoreWriter, key string, e *entry) error {
	bytes, err := vom.Encode(e)
	if err != nil {
		return convertError(err)
	}
	if err := stw.Put([]byte(key), bytes); err != nil {
		return convertError(err)
	}
	return nil
}

func delete(stw istore.StoreWriter, key string) error {
	if err := stw.Delete([]byte(key)); err != nil {
		return convertError(err)
	}
	return nil
}

func (e *entry) checkVersion(version string) error {
	newVersion := strconv.FormatUint(e.Version, 10)
	if version != newVersion {
		return verror.NewErrBadVersion(nil)
	}
	return nil
}

func convertError(err error) error {
	return verror.Convert(verror.IDAction{}, nil, err)
}

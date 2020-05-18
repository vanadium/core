// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Implements a map-based store substitute that implements the legacy store API.
package fs

import (
	"encoding/gob"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/services/application"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/x/lib/set"
	"v.io/x/ref/services/profile"
)

// TODO(rjkroege@google.com) Switch Memstore to the mid-August 2014
// style store API.

const pkgPath = "v.io/x/ref/services/internal/fs"

// Errors
var (
	ErrNoRecursiveCreateTransaction = verror.Register(pkgPath+".ErrNoRecursiveCreateTransaction", verror.NoRetry, "{1:}{2:} recursive CreateTransaction() not permitted{:_}")
	ErrDoubleCommit                 = verror.Register(pkgPath+".ErrDoubleCommit", verror.NoRetry, "{1:}{2:} illegal attempt to commit previously committed or abandonned transaction{:_}")
	ErrAbortWithoutTransaction      = verror.Register(pkgPath+".ErrAbortWithoutTransaction", verror.NoRetry, "{1:}{2:} illegal attempt to abort non-existent transaction{:_}")
	ErrWithoutTransaction           = verror.Register(pkgPath+".ErrRemoveWithoutTransaction", verror.NoRetry, "{1:}{2:} call without a transaction{:_}")
	ErrNotInMemStore                = verror.Register(pkgPath+".ErrNotInMemStore", verror.NoRetry, "{1:}{2:} not in Memstore{:_}")
	ErrUnsupportedType              = verror.Register(pkgPath+".ErrUnsupportedType", verror.NoRetry, "{1:}{2:} attempted Put to Memstore of unsupported type{:_}")
	ErrChildrenWithoutLock          = verror.Register(pkgPath+".ErrChildrenWithoutLock", verror.NoRetry, "{1:}{2:} Children() without a lock{:_}")

	errTempFileFailed      = verror.Register(pkgPath+".errTempFileFailed", verror.NoRetry, "{1:}{2:} TempFile({3}, {4}) failed{:_}")
	errCantCreate          = verror.Register(pkgPath+".errCantCreate", verror.NoRetry, "{1:}{2:} File ({3}) could not be created ({4}){:_}")
	errCantOpen            = verror.Register(pkgPath+".errCantOpen", verror.NoRetry, "{1:}{2:} File ({3}) could not be opened ({4}){:_}")
	errDecodeFailedBadData = verror.Register(pkgPath+".errDecodeFailedBadData", verror.NoRetry, "{1:}{2:} Decode() failed: data format mismatch or backing file truncated{:_}")

	errEncodeFailed       = verror.Register(pkgPath+".errEncodeFailed", verror.NoRetry, "{1:}{2:} Encode() failed{:_}")
	errFileSystemError    = verror.Register(pkgPath+".errFileSystemError", verror.NoRetry, "{1:}{2:} File system operation failed{:_}")
	errFormatUpgradeError = verror.Register(pkgPath+".errFormatUpgradeError", verror.NoRetry, "{1:}{2:} File format upgrading failed{:_}")
)

// Memstore contains the state of the memstore. It supports a single
// transaction at a time. The current state of a Memstore under a
// transactional name binding is the contents of puts then the contents
// of (data - removes). puts and removes will be empty at the beginning
// of a transaction and after an Unlock operation.
type Memstore struct {
	sync.Mutex
	persistedFile              string
	haveTransactionNameBinding bool
	locked                     bool
	data                       map[string]interface{}
	puts                       map[string]interface{}
	removes                    map[string]struct{}
}

const (
	startingMemstoreSize  = 10
	transactionNamePrefix = "memstore-transaction"

	// A memstore is a serialized Go map. A GOB-encoded map using Go 1.4
	// cannot begin with this byte. Consequently, simplestore writes this
	// magic byte to the start of a vom file. Its absence at the
	// beginning of the file indicates that the memstore file is using
	// the GOB legacy encoding.
	vomGobMagicByte = 0xF0
)

var keyExists = struct{}{}

// TODO(rjkroege): Simplestore used GOB for its persistence
// layer. However, now, it uses VOM to store persisted values and
// automatically converts GOB format files to VOM-compatible on load.
// At some point in the future, it may be possible to simplify the
// implementation by removing support for loading files in the
// now legacy GOB format.
type applicationEnvelope struct {
	Title             string
	Args              []string
	Binary            application.SignedFile
	Publisher         security.WireBlessings
	Env               []string
	Packages          application.Packages
	Restarts          int32
	RestartTimeWindow time.Duration
}

// This function is needed only to support existing serialized data and
// can be removed in a future release.
func translateFromGobEncodeable(in interface{}) (interface{}, error) {
	env, ok := in.(applicationEnvelope)
	if !ok {
		return in, nil
	}
	// Have to roundtrip through vom to convert from WireBlessings to Blessings.
	// This may seem silly, but this whole translation business is silly too :)
	// and will go away once we switch this package to using 'vom' instead of 'gob'.
	// So for now, live with the funkiness.
	bytes, err := vom.Encode(env.Publisher)
	if err != nil {
		return nil, err
	}
	var publisher security.Blessings
	if err := vom.Decode(bytes, &publisher); err != nil {
		return nil, err
	}
	return application.Envelope{
		Title:             env.Title,
		Args:              env.Args,
		Binary:            env.Binary,
		Publisher:         publisher,
		Env:               env.Env,
		Packages:          env.Packages,
		Restarts:          env.Restarts,
		RestartTimeWindow: env.RestartTimeWindow,
	}, nil
}

// The implementation of set requires gob instead of json.
func init() {
	gob.Register(profile.Specification{})
	gob.Register(applicationEnvelope{})
	gob.Register(access.Permissions{})
	// Ensure that no fields have been added to application.Envelope,
	// because if so, then applicationEnvelope defined in this package
	// needs to change
	if n := reflect.TypeOf(application.Envelope{}).NumField(); n != 8 {
		panic("It appears that fields have been added to or removed from application.Envelope before the hack in this file around gob-encodeability was removed. Please also update applicationEnvelope, translateToGobEncodeable and translateToGobDecodeable in this file")
	}
}

// isVOM returns true if the file is a VOM-format file.
func isVOM(file io.ReadSeeker) (bool, error) {
	oneByte := make([]byte, 1)
	c, err := file.Read(oneByte)
	for c == 0 && err == nil {
		c, err = file.Read(oneByte)
	}

	if c > 0 {
		if oneByte[0] == vomGobMagicByte {
			return true, nil
		} else {
			if _, err := file.Seek(0, 0); err != nil {
				return false, verror.New(errFileSystemError, nil, err)
			}
			return false, nil
		}
	}
	return false, err
}

func nonEmptyExists(fileName string) (bool, error) {
	fi, serr := os.Stat(fileName)
	if os.IsNotExist(serr) {
		return false, nil
	} else if serr != nil {
		return false, serr
	}
	if fi.Size() > 0 {
		return true, nil
	}
	return false, nil
}

func convertFileToVomIfNecessary(fileName string) error {
	// Open the legacy file.
	file, err := os.Open(fileName)
	if err != nil {
		return verror.New(errCantOpen, nil, fileName, err)
	}
	defer file.Close()

	// VOM files don't need conversion.
	if is, err := isVOM(file); is || err != nil {
		return err
	}

	// Decode the legacy GOB file
	decoder := gob.NewDecoder(file)
	data := make(map[string]interface{}, startingMemstoreSize)
	if err := decoder.Decode(&data); err != nil {
		return verror.New(errDecodeFailedBadData, nil, err)
	}

	// Update GOB file to VOM format in memory.
	for k, v := range data {
		tv, err := translateFromGobEncodeable(v)
		if err != nil {
			return verror.New(errFormatUpgradeError, nil, err)
		}
		data[k] = tv
	}

	ms := &Memstore{
		data:          data,
		persistedFile: fileName,
	}
	if err := ms.persist(); err != nil {
		return err
	}
	return nil
}

// NewMemstore persists the Memstore to os.TempDir() if no file is
// configured.
func NewMemstore(configuredPersistentFile string) (*Memstore, error) {
	data := make(map[string]interface{}, startingMemstoreSize)
	if configuredPersistentFile == "" {
		f, err := ioutil.TempFile(os.TempDir(), "memstore-vom")
		if err != nil {
			return nil, verror.New(errTempFileFailed, nil, os.TempDir(), "memstore-vom", err)
		}
		f.Close()
		configuredPersistentFile = f.Name()
	}

	// Empty or non-existent files.
	nee, err := nonEmptyExists(configuredPersistentFile)
	if err != nil {
		return nil, verror.New(errCantCreate, nil, configuredPersistentFile, err)
	}
	if !nee {
		// Ensure that we can create a file.
		file, cerr := os.Create(configuredPersistentFile)
		if cerr != nil {
			return nil, verror.New(errCantCreate, nil, configuredPersistentFile, err, cerr)
		}
		file.Close()
		return &Memstore{
			data:          data,
			persistedFile: configuredPersistentFile,
		}, nil
	}

	// Convert a non-empty GOB file into VOM format.
	if err := convertFileToVomIfNecessary(configuredPersistentFile); err != nil {
		return nil, err
	}

	file, err := os.Open(configuredPersistentFile)
	if err != nil {
		return nil, verror.New(errCantOpen, nil, configuredPersistentFile, err)
	}
	// Skip past the magic byte that identifies this as a VOM format file.
	if _, err := file.Seek(1, 0); err != nil {
		return nil, verror.New(errFileSystemError, nil, err)
	}

	decoder := vom.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		return nil, verror.New(errDecodeFailedBadData, nil, err)
	}

	return &Memstore{
		data:          data,
		persistedFile: configuredPersistentFile,
	}, nil
}

type MemstoreObject interface {
	Remove(_ interface{}) error
	Exists(_ interface{}) (bool, error)
}

type boundObject struct {
	path  string
	ms    *Memstore
	Value interface{}
}

// BindObject sets the path string for subsequent operations.
func (ms *Memstore) BindObject(path string) *boundObject {
	pathParts := strings.SplitN(path, "/", 2)
	if pathParts[0] == transactionNamePrefix {
		ms.haveTransactionNameBinding = true
	} else {
		ms.haveTransactionNameBinding = false
	}
	return &boundObject{path: pathParts[1], ms: ms}
}

func (ms *Memstore) removeChildren(path string) bool {
	deleted := false
	for k := range ms.data {
		if strings.HasPrefix(k, path) {
			deleted = true
			ms.removes[k] = keyExists
		}
	}
	for k := range ms.puts {
		if strings.HasPrefix(k, path) {
			deleted = true
			delete(ms.puts, k)
		}
	}
	return deleted
}

type Transaction interface {
	CreateTransaction(_ interface{}) (string, error)
	Commit(_ interface{}) error
}

// BindTransactionRoot on a Memstore always operates over the
// entire Memstore. As a result, the root parameter is ignored.
func (ms *Memstore) BindTransactionRoot(_ string) Transaction {
	return ms
}

// BindTransaction on a Memstore can only use the single Memstore
// transaction.
func (ms *Memstore) BindTransaction(_ string) Transaction {
	return ms
}

func (ms *Memstore) newTransactionState() {
	ms.puts = make(map[string]interface{}, startingMemstoreSize)
	ms.removes = make(map[string]struct{}, startingMemstoreSize)
}

func (ms *Memstore) clearTransactionState() {
	ms.puts = nil
	ms.removes = nil
}

// Unlock abandons an in-progress transaction before releasing the lock.
func (ms *Memstore) Unlock() {
	ms.locked = false
	ms.clearTransactionState()
	ms.Mutex.Unlock()
}

// Lock acquires a lock and caches the state of the lock.
func (ms *Memstore) Lock() {
	ms.Mutex.Lock()
	ms.clearTransactionState()
	ms.locked = true
}

// CreateTransaction requires the caller to acquire a lock on the Memstore.
func (ms *Memstore) CreateTransaction(_ interface{}) (string, error) {
	if ms.puts != nil || ms.removes != nil {
		return "", verror.New(ErrNoRecursiveCreateTransaction, nil)
	}
	ms.newTransactionState()
	return transactionNamePrefix, nil
}

// Commit updates the store and persists the result.
func (ms *Memstore) Commit(_ interface{}) error {
	if !ms.locked || ms.puts == nil || ms.removes == nil {
		return verror.New(ErrDoubleCommit, nil)
	}
	for k, v := range ms.puts {
		ms.data[k] = v
	}
	for k := range ms.removes {
		delete(ms.data, k)
	}
	return ms.persist()
}

func (ms *Memstore) Abort(_ interface{}) error {
	if !ms.locked {
		return verror.New(ErrAbortWithoutTransaction, nil)
	}
	return nil
}

func (o *boundObject) Remove(_ interface{}) error {
	if !o.ms.locked {
		return verror.New(ErrWithoutTransaction, nil, "Remove()")
	}

	if _, pendingRemoval := o.ms.removes[o.path]; pendingRemoval {
		return verror.New(ErrNotInMemStore, nil, o.path)
	}

	_, found := o.ms.data[o.path]
	if !found && !o.ms.removeChildren(o.path) {
		return verror.New(ErrNotInMemStore, nil, o.path)
	}
	delete(o.ms.puts, o.path)
	o.ms.removes[o.path] = keyExists
	return nil
}

// transactionExists implements Exists() for bound names that have the
// transaction prefix.
func (o *boundObject) transactionExists() bool {
	// Determine if the bound name point to a real object.
	_, inBase := o.ms.data[o.path]
	_, inPuts := o.ms.puts[o.path]
	_, inRemoves := o.ms.removes[o.path]

	// not yet committed.
	if inPuts || (inBase && !inRemoves) {
		return true
	}

	// The bound names might be a prefix of the path for a real object. For
	// example, BindObject("/test/a"). Put(o) creates a real object o at path
	/// test/a so the code above will cause BindObject("/test/a").Exists() to
	// return true. Testing this is however not sufficient because
	// BindObject(any prefix of on object path).Exists() needs to also be
	// true. For example, here BindObject("/test").Exists() is true.
	//
	// Consequently, transactionExists scans all object names in the Memstore
	// to determine if any of their names have the bound name as a prefix.
	// Puts take precedence over removes so we scan it first.

	for k := range o.ms.puts {
		if strings.HasPrefix(k, o.path) {
			return true
		}
	}

	// Then we scan data for matches and verify that at least one of the
	// object names with the bound prefix have not been removed.

	for k := range o.ms.data {
		if _, inRemoves := o.ms.removes[k]; strings.HasPrefix(k, o.path) && !inRemoves {
			return true
		}
	}
	return false
}

func (o *boundObject) Exists(_ interface{}) (bool, error) {
	if o.ms.haveTransactionNameBinding {
		return o.transactionExists(), nil
	} else {
		_, inBase := o.ms.data[o.path]
		if inBase {
			return true, nil
		}
		for k := range o.ms.data {
			if strings.HasPrefix(k, o.path) {
				return true, nil
			}
		}
	}
	return false, nil
}

// transactionBoundGet implements Get while the bound name has the
// transaction prefix.
func (o *boundObject) transactionBoundGet() (*boundObject, error) {
	bv, inBase := o.ms.data[o.path]
	_, inRemoves := o.ms.removes[o.path]
	pv, inPuts := o.ms.puts[o.path]

	found := inPuts || (inBase && !inRemoves)
	if !found {
		return nil, verror.New(ErrNotInMemStore, nil, o.path)
	}

	if inPuts {
		o.Value = pv
	} else {
		o.Value = bv
	}
	var err error
	if o.Value, err = translateFromGobEncodeable(o.Value); err != nil {
		return nil, err
	}
	return o, nil
}

func (o *boundObject) bareGet() (*boundObject, error) {
	bv, inBase := o.ms.data[o.path]

	if !inBase {
		return nil, verror.New(ErrNotInMemStore, nil, o.path)
	}
	o.Value = bv
	return o, nil
}

func (o *boundObject) Get(_ interface{}) (*boundObject, error) {
	if o.ms.haveTransactionNameBinding {
		return o.transactionBoundGet()
	} else {
		return o.bareGet()
	}
}

func (o *boundObject) Put(_ interface{}, envelope interface{}) (*boundObject, error) {
	if !o.ms.locked {
		return nil, verror.New(ErrWithoutTransaction, nil, "Put()")
	}
	switch v := envelope.(type) {
	case application.Envelope, profile.Specification, access.Permissions:
		o.ms.puts[o.path] = v
		delete(o.ms.removes, o.path)
		o.Value = o.path
		return o, nil
	default:
		return o, verror.New(ErrUnsupportedType, nil)
	}
}

func (o *boundObject) Children() ([]string, error) {
	if !o.ms.locked {
		return nil, verror.New(ErrChildrenWithoutLock, nil)
	}
	found := false
	childrenSet := make(map[string]struct{})
	for k := range o.ms.data {
		if strings.HasPrefix(k, o.path) || o.path == "" {
			name := strings.TrimPrefix(k, o.path)
			// Found the object itself.
			if len(name) == 0 {
				found = true
				continue
			}
			// This was only a prefix match, not what we're looking for.
			if name[0] != '/' && o.path != "" {
				continue
			}
			found = true
			name = strings.TrimLeft(name, "/")
			if idx := strings.Index(name, "/"); idx != -1 {
				name = name[:idx]
			}
			childrenSet[name] = keyExists
		}
	}
	if !found {
		return nil, verror.New(verror.ErrNoExist, nil, o.path)
	}
	children := set.String.ToSlice(childrenSet)
	sort.Strings(children)
	return children, nil
}

// persist() writes the state of the Memstore to persistent storage.
func (ms *Memstore) persist() error {
	file, err := ioutil.TempFile(os.TempDir(), "memstore-persisting")
	if err != nil {
		return verror.New(errTempFileFailed, nil, os.TempDir(), "memstore-persisting", err)
	}
	defer file.Close()
	defer os.Remove(file.Name())

	// Mark this VOM file with the VOM file format magic byte.
	if _, err := file.Write([]byte{byte(vomGobMagicByte)}); err != nil {
		return err
	}
	enc := vom.NewEncoder(file)
	if err := enc.Encode(ms.data); err != nil {
		return verror.New(errEncodeFailed, nil, err)
	}
	ms.clearTransactionState()

	if err := os.Rename(file.Name(), ms.persistedFile); err != nil {
		return verror.New(errEncodeFailed, nil, err)
	}
	return nil
}

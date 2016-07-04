// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package syncbase defines the Syncbase client library.
package syncbase

// Namespace: <serviceName>/<encDbId>/<encCxId>/<encRowKey>, where:
//   <encDbId> is encode(<dbId>), where <dbId> is <appBlessing>,<dbName>
//   <encCxId> is encode(<cxId>), where <cxId> is <userBlessing>,<cxName>
// (Note that blessing strings cannot contain ",".)

// NOTE(sadovsky): Various methods below may end up needing additional options.
// One can add options to a Go method in a backwards-compatible way by making
// the method variadic.

// TODO(sadovsky): Document the access control policy for every method where
// it's not obvious.

import (
	"fmt"
	"time"

	"v.io/v23/context"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/services/watch"
	"v.io/v23/syncbase/util"
	"v.io/v23/verror"
	"v.io/v23/vom"
)

// Service represents a Vanadium Syncbase service.
// Use NewService to get a Service.
type Service interface {
	// FullName returns the object name (encoded) of this Service.
	FullName() string

	// Database returns the Database with the given relative name.
	// The app blessing is derived from the context.
	// TODO(sadovsky): Revisit API for schema stuff.
	Database(ctx *context.T, name string, schema *Schema) Database

	// DatabaseForId returns the Database with the given app blessing and name.
	DatabaseForId(id wire.Id, schema *Schema) Database

	// ListDatabases returns a list of all Database ids that the caller is allowed
	// to see. The list is sorted by blessing, then by name.
	ListDatabases(ctx *context.T) ([]wire.Id, error)

	// SetPermissions and GetPermissions are included from the AccessController
	// interface.
	util.AccessController
}

// DatabaseHandle is the set of methods that work both with and without a batch.
// It allows clients to pass the handle to helper methods that are
// batch-agnostic.
type DatabaseHandle interface {
	// Id returns the id of this DatabaseHandle.
	Id() wire.Id

	// FullName returns the object name (encoded) of this DatabaseHandle.
	FullName() string

	// Collection returns the Collection with the given relative name.
	// The user blessing is derived from the context.
	Collection(ctx *context.T, name string) Collection

	// CollectionForId returns the Collection with the given user blessing and
	// name.
	CollectionForId(id wire.Id) Collection

	// ListCollections returns a list of all Collection ids that the caller is
	// allowed to see. The list is sorted by blessing, then by name.
	ListCollections(ctx *context.T) ([]wire.Id, error)

	// Exec executes a syncQL query.
	//
	// A value must be provided for every positional parameter ('?' placeholder)
	// in the query.
	//
	// For select statements:
	// If no error is returned, Exec returns an array of headers (i.e. column
	// names) and a result stream with an array of values for each row that
	// matches the query. The number of values returned in each row of the
	// result stream will match the size of the headers array.
	//
	// For delete statements:
	// If no error is returned, Exec returns an array of headers with exactly one
	// column, "Count", and a result stream with an array containing a single
	// element of type vdl.Int64. The value represents the number of rows deleted.
	//
	// Concurrency semantics: It is legal to perform writes concurrently with
	// Exec. The returned stream reads from a consistent snapshot taken at the
	// time of the RPC (or at the time of BeginBatch, if in a batch), and will not
	// reflect subsequent writes to keys not yet reached by the stream.
	Exec(ctx *context.T, query string, params ...interface{}) ([]string, ResultStream, error)

	// GetResumeMarker returns a ResumeMarker that points to the current end of
	// the event log.
	GetResumeMarker(ctx *context.T) (watch.ResumeMarker, error)
}

// Database represents a set of Collections.
type Database interface {
	DatabaseHandle

	// Create creates this Database.
	// If perms is nil, the user blessing derived from the context is given all
	// permissions.
	Create(ctx *context.T, perms access.Permissions) error

	// Destroy destroys this Database, permanently removing all of its data.
	// TODO(sadovsky): Specify what happens to syncgroups.
	Destroy(ctx *context.T) error

	// Exists returns true only if this Database exists. Insufficient permissions
	// cause Exists to return false instead of an error.
	Exists(ctx *context.T) (bool, error)

	// BeginBatch creates a new batch. Instead of calling this function directly,
	// clients are encouraged to use the RunInBatch() helper function, which
	// detects "concurrent batch" errors and handles retries internally.
	//
	// Default concurrency semantics:
	// - Reads (e.g. gets, scans) inside a batch operate over a consistent
	//   snapshot taken during BeginBatch(), and will see the effects of prior
	//   writes performed inside the batch.
	// - Commit() may fail with ErrConcurrentBatch, indicating that after
	//   BeginBatch() but before Commit(), some concurrent routine wrote to a key
	//   that matches a key or row-range read inside this batch.
	// - Other methods will never fail with error ErrConcurrentBatch, even if it
	//   is known that Commit() will fail with this error.
	//
	// Once a batch has been committed or aborted, subsequent method calls will
	// fail with no effect.
	//
	// Concurrency semantics can be configured using BatchOptions.
	// TODO(sadovsky): Use varargs for options.
	BeginBatch(ctx *context.T, opts wire.BatchOptions) (BatchDatabase, error)

	// SetPermissions and GetPermissions are included from the AccessController
	// interface.
	util.AccessController

	// Watch allows a client to watch for updates to the database. At least one
	// pattern must be specified. For each watch request, the client will receive
	// a reliable stream of watch events without reordering. Only rows matching at
	// least one of the patterns are returned. Rows in collections with no Read
	// access are also filtered out.
	//
	// If a nil ResumeMarker is provided, the WatchStream will begin with a Change
	// batch containing the initial state, always starting with an empty update
	// for the root entity. Otherwise, the WatchStream will contain only changes
	// since the provided ResumeMarker.
	// See watch.GlobWatcher for a detailed explanation of the behavior.
	Watch(ctx *context.T, resumeMarker watch.ResumeMarker, patterns []wire.CollectionRowPattern) WatchStream

	// Syncgroup returns the Syncgroup with the given relative name.
	// The user blessing is derived from the context.
	Syncgroup(ctx *context.T, name string) Syncgroup

	// SyncgroupForId returns the Syncgroup with the given user blessing and name.
	SyncgroupForId(id wire.Id) Syncgroup

	// ListSyncgroups returns all Syncgroups attached to this database.
	ListSyncgroups(ctx *context.T) ([]wire.Id, error)

	// CreateBlob creates a new blob and returns a handle to it.
	CreateBlob(ctx *context.T) (Blob, error)

	// Blob returns a handle to the blob with the given BlobRef.
	Blob(br wire.BlobRef) Blob

	// EnforceSchema compares the current schema version of the database with the
	// schema version provided when creating this database handle, and updates the
	// schema metadata if required.
	//
	// This method also registers a conflict resolver with Syncbase to receive
	// conflicts. Note: schema can be nil, in which case this method should not be
	// called and the caller is responsible for maintaining schema sanity.
	EnforceSchema(ctx *context.T) error

	// PauseSync pauses sync for this database. Incoming sync, as well as outgoing
	// sync of subsequent writes, will be disabled until ResumeSync is called.
	// PauseSync is idempotent.
	PauseSync(ctx *context.T) error

	// ResumeSync resumes sync for this database. ResumeSync is idempotent.
	ResumeSync(ctx *context.T) error

	// Close cleans up any state associated with this database handle, including
	// closing the conflict resolution stream (if open).
	Close()
}

// BatchDatabase is a handle to a set of reads and writes to the database that
// should be considered an atomic unit. See BeginBatch() for concurrency
// semantics.
// TODO(sadovsky): If/when needed, add a CommitWillFail() method so that clients
// can avoid doing extra work inside a doomed batch.
// TODO(ivanpi): Document Abort-after-failed-Commit semantics and update all
// client RunInBatch methods.
type BatchDatabase interface {
	DatabaseHandle

	// Commit persists the pending changes to the database.
	// If the batch is readonly, Commit() will fail with ErrReadOnlyBatch; Abort()
	// should be used instead.
	Commit(ctx *context.T) error

	// Abort notifies the server that any pending changes can be discarded.
	// It is not strictly required, but it may allow the server to release locks
	// or other resources sooner than if it was not called.
	Abort(ctx *context.T) error
}

// Collection represents a set of Rows.
//
// TODO(sadovsky): Currently we provide Get/Put/Delete methods on both
// Collection and Row, because we're not sure which will feel more natural.
// Eventually, we'll need to pick one.
type Collection interface {
	// Id returns the id of this Collection.
	Id() wire.Id

	// FullName returns the object name (encoded) of this Collection.
	FullName() string

	// Exists returns true only if this Collection exists. Insufficient
	// permissions cause Exists to return false instead of an error.
	// TODO(ivanpi): Exists may fail with an error if higher levels of hierarchy
	// do not exist.
	Exists(ctx *context.T) (bool, error)

	// Create creates this Collection.
	// If perms is nil, the user blessing derived from the context is given all
	// permissions.
	Create(ctx *context.T, perms access.Permissions) error

	// Destroy destroys this Collection, permanently removing all of its data.
	// TODO(sadovsky): Specify what happens to syncgroups.
	Destroy(ctx *context.T) error

	// GetPermissions returns the current Permissions for the Collection.
	// The Read bit on the ACL does not affect who this Collection's rows are
	// synced to; all members of syncgroups that include this Collection will
	// receive the rows in this Collection. It only determines which clients
	// are allowed to retrieve the value using a Syncbase RPC.
	GetPermissions(ctx *context.T) (access.Permissions, error)

	// SetPermissions replaces the current Permissions for the Collection.
	SetPermissions(ctx *context.T, perms access.Permissions) error

	// Row returns the Row with the given key.
	Row(key string) Row

	// Get loads the value stored under the given key into the given value.
	// If the given value's type does not match the stored value's type, Get
	// will return an error. Expected usage:
	//     var value MyType
	//     if err := cx.Get(ctx, key, &value); err != nil {
	//       return err
	//     }
	Get(ctx *context.T, key string, value interface{}) error

	// Put writes the given value to this Collection under the given key.
	// TODO(kash): Can VOM handle everything that satisfies interface{}?
	// Need to talk to Todd.
	// TODO(sadovsky): Maybe distinguish insert from update (and also offer
	// upsert) so that last-one-wins can have deletes trump updates.
	Put(ctx *context.T, key string, value interface{}) error

	// Delete deletes the row for the given key.
	Delete(ctx *context.T, key string) error

	// DeleteRange deletes all rows in the given half-open range [start, limit).
	// If limit is "", all rows with keys >= start are included.
	// TODO(sadovsky): Document how this deletion is considered during conflict
	// detection: is it considered as a range deletion, or as a bunch of point
	// deletions?
	// See helpers Prefix(), Range(), SingleRow().
	DeleteRange(ctx *context.T, r RowRange) error

	// Scan returns all rows in the given half-open range [start, limit). If limit
	// is "", all rows with keys >= start are included.
	// Concurrency semantics: It is legal to perform writes concurrently with
	// Scan. The returned stream reads from a consistent snapshot taken at the
	// time of the RPC (or at the time of BeginBatch, if in a batch), and will not
	// reflect subsequent writes to keys not yet reached by the stream.
	// See helpers Prefix(), Range(), SingleRow().
	Scan(ctx *context.T, r RowRange) ScanStream
}

// Row represents a single row in a Collection.
type Row interface {
	// Key returns the key for this Row.
	Key() string

	// FullName returns the object name (encoded) of this Row.
	FullName() string

	// Exists returns true only if this Row exists. Insufficient permissions cause
	// Exists to return false instead of an error.
	// TODO(ivanpi): Exists may fail with an error if higher levels of hierarchy
	// do not exist.
	Exists(ctx *context.T) (bool, error)

	// Get loads the value stored in this Row into the given value. If the given
	// value's type does not match the stored value's type, Get will return an
	// error. Expected usage:
	//     var value MyType
	//     if err := row.Get(ctx, &value); err != nil {
	//       return err
	//     }
	Get(ctx *context.T, value interface{}) error

	// Put writes the given value for this Row.
	Put(ctx *context.T, value interface{}) error

	// Delete deletes this Row.
	Delete(ctx *context.T) error
}

// Stream is an interface for iterating through a collection of elements.
type Stream interface {
	// Advance stages an element so the client can retrieve it. Advance returns
	// true iff there is an element to retrieve. The client must call Advance
	// before retrieving the element. The client must call Cancel if it does not
	// iterate through all elements (i.e. until Advance returns false).
	// Advance may block if an element is not immediately available.
	Advance() bool

	// Err returns a non-nil error iff the stream encountered any errors. Err does
	// not block.
	Err() error

	// Cancel notifies the stream provider that it can stop producing elements.
	// The client must call Cancel if it does not iterate through all elements
	// (i.e. until Advance returns false). Cancel is idempotent and can be called
	// concurrently with a goroutine that is iterating via Advance.
	// Cancel causes Advance to subsequently return false. Cancel does not block.
	Cancel()
}

// ScanStream is an interface for iterating through a collection of key-value
// pairs.
type ScanStream interface {
	Stream

	// Key returns the key of the element that was staged by Advance.
	// Key may panic if Advance returned false or was not called at all.
	// Key does not block.
	Key() string

	// Value returns the value of the element that was staged by Advance, or an
	// error if the value could not be decoded.
	// Value may panic if Advance returned false or was not called at all.
	// Value does not block.
	Value(value interface{}) error
}

// ResultStream is an interface for iterating through Exec query results.
type ResultStream interface {
	Stream

	// ResultCount returns the number of results for the stream element
	// prepared by the most recent call to Advance().  Requires that the
	// last call to Advance() was successful.
	ResultCount() int

	// ResultValue loads the result numbered i into the given value.
	// Requires 0 <= i < ResultCount(), and that the last call to Advance()
	// was successful.
	// Errors represent possible decoding errors for individual values,
	// rather than errors that would necessarily terminate the stream.
	Result(i int, value interface{}) error
}

// WatchStream is an interface for receiving database updates.
type WatchStream interface {
	Stream

	// Change returns the element that was staged by Advance.
	// Change may panic if Advance returned false or was not called at all.
	// Change does not block.
	Change() WatchChange
}

// ChangeType denotes the type of the change: Put or Delete.
type ChangeType uint32

const (
	PutChange ChangeType = iota
	DeleteChange
)

// EntityType denotes the type of the changed entity: Root, Collection, or Row.
// TODO(ivanpi): Consider adding syncgroup metadata and other types.
type EntityType uint32

const (
	EntityRoot EntityType = iota
	EntityCollection
	EntityRow
)

// WatchChange represents a change to a watched entity.
type WatchChange struct {
	// EntityType is the type of the entity - Root, Collection, or Row.
	EntityType EntityType

	// Collection is the id of the collection that was changed or contains the
	// changed row. Has zero value if EntityType is not Collection or Row.
	Collection wire.Id

	// Row is the key of the changed row. Empty if EntityType is not Row.
	Row string

	// ChangeType describes the type of the change, depending on the EntityType:
	// - for EntityRow:
	//   * PutChange: the row exists in the collection, and Value can be called to
	//     obtain the new value for this row.
	//   * DeleteChange: the row was removed from the collection.
	// - for EntityCollection:
	//   * PutChange: the collection exists, and CollectionInfo can be called to
	//     obtain the collection info.
	//   * DeleteChange: the collection was destroyed.
	// - for EntityRoot:
	//   * PutChange: appears as the first (possibly only) change in the initial
	//     state batch, only if watching from an empty ResumeMarker. This is the
	//     only situation where an EntityRoot appears.
	ChangeType ChangeType

	// value is the new value for the row for EntityRow PutChanges, an encoded
	// StoreChangeCollectionInfo value for EntityCollection PutChanges, or nil
	// otherwise.
	value *vom.RawBytes

	// ResumeMarker provides a compact representation of all the messages that
	// have been received by the caller for the given Watch call.
	// This marker can be provided in the Request message to allow the caller
	// to resume the stream watching at a specific point without fetching the
	// initial state.
	ResumeMarker watch.ResumeMarker

	// FromSync indicates whether the change came from sync. If FromSync is false,
	// then the change originated from the local device.
	FromSync bool

	// If true, this WatchChange is followed by more WatchChanges that are in the
	// same batch as this WatchChange.
	Continued bool
}

// Value decodes the new value of the watched element. Panics if the change type
// is DeleteChange or the entity is not a Row.
func (c *WatchChange) Value(value interface{}) error {
	if c.ChangeType != PutChange {
		panic("invalid change type")
	}
	if c.EntityType != EntityRow {
		panic("invalid entity type")
	}
	return c.value.ToValue(value)
}

// CollectionInfo returns the collection info containing permissions that the
// watcher has on the collection. Panics if the change type is DeleteChange or
// the entity is not a Collection.
func (c *WatchChange) CollectionInfo() *wire.StoreChangeCollectionInfo {
	if c.ChangeType != PutChange {
		panic("invalid change type")
	}
	if c.EntityType != EntityCollection {
		panic("invalid entity type")
	}
	ci := &wire.StoreChangeCollectionInfo{}
	if err := c.value.ToValue(ci); err != nil {
		// This should never panic since we verify the collection info is decodable
		// when constructing the WatchChange.
		panic(fmt.Errorf("ToValue StoreChangeCollectionInfo failed: %v, RawBytes: %#v", err, c.value))
	}
	return ci
}

// Syncgroup is the interface for a syncgroup in the store.
type Syncgroup interface {
	// Create creates a new syncgroup with the given spec.
	//
	// Requires: Client must have at least Read access on the Database; all
	// Collections specified in prefixes must exist; Client must have at least
	// Read access on each of the Collection ACLs.
	Create(ctx *context.T, spec wire.SyncgroupSpec, myInfo wire.SyncgroupMemberInfo) error

	// Join joins a syncgroup.
	//
	// Requires: Client must have at least Read access on the Database and on the
	// syncgroup ACL.
	Join(ctx *context.T, syncbaseName string, expectedSyncbaseBlessings []string, myInfo wire.SyncgroupMemberInfo) (wire.SyncgroupSpec, error)

	// Leave leaves the syncgroup. Previously synced data will continue
	// to be available.
	//
	// Requires: Client must have at least Read access on the Database.
	Leave(ctx *context.T) error

	// Destroy destroys the syncgroup. Previously synced data will
	// continue to be available to all members.
	//
	// Requires: Client must have at least Read access on the Database, and must
	// have Admin access on the syncgroup ACL.
	Destroy(ctx *context.T) error

	// Eject ejects a member from the syncgroup. The ejected member
	// will not be able to sync further, but will retain any data it has already
	// synced.
	//
	// Requires: Client must have at least Read access on the Database, and must
	// have Admin access on the syncgroup ACL.
	Eject(ctx *context.T, member string) error

	// GetSpec gets the syncgroup spec. version allows for atomic
	// read-modify-write of the spec - see comment for SetSpec.
	//
	// Requires: Client must have at least Read access on the Database and on the
	// syncgroup ACL.
	GetSpec(ctx *context.T) (spec wire.SyncgroupSpec, version string, err error)

	// SetSpec sets the syncgroup spec. version may be either empty or
	// the value from a previous Get. If not empty, Set will only succeed if the
	// current version matches the specified one.
	//
	// Requires: Client must have at least Read access on the Database, and must
	// have Admin access on the syncgroup ACL.
	SetSpec(ctx *context.T, spec wire.SyncgroupSpec, version string) error

	// GetMembers gets the info objects for members of the syncgroup.
	//
	// Requires: Client must have at least Read access on the Database and on the
	// syncgroup ACL.
	GetMembers(ctx *context.T) (map[string]wire.SyncgroupMemberInfo, error)

	// Id returns the relative syncgroup name and blessing.
	Id() wire.Id
}

// Blob is the interface for a Blob in the store.
type Blob interface {
	// Ref returns Syncbase's BlobRef for this blob.
	Ref() wire.BlobRef

	// Put appends the byte stream to the blob.
	Put(ctx *context.T) (BlobWriter, error)

	// Commit marks the blob as immutable.
	Commit(ctx *context.T) error

	// Size returns the count of bytes written as part of the blob
	// (committed or uncommitted).
	Size(ctx *context.T) (int64, error)

	// Delete locally deletes the blob (committed or uncommitted).
	Delete(ctx *context.T) error

	// Get returns the byte stream from a committed blob starting at offset.
	Get(ctx *context.T, offset int64) (BlobReader, error)

	// Fetch initiates fetching a blob if not locally found. priority
	// controls the network priority of the blob. Higher priority blobs are
	// fetched before the lower priority ones. However an ongoing blob
	// transfer is not interrupted. Status updates are streamed back to the
	// client as fetch is in progress.
	Fetch(ctx *context.T, priority uint64) (BlobStatus, error)

	// Pin locally pins the blob so that it is not evicted.
	Pin(ctx *context.T) error

	// Unpin locally unpins the blob so that it can be evicted if needed.
	Unpin(ctx *context.T) error

	// Keep locally caches the blob with the specified rank. Lower
	// ranked blobs are more eagerly evicted.
	Keep(ctx *context.T, rank uint64) error
}

// BlobWriter is an interface for putting a blob.
type BlobWriter interface {
	// Send places the bytes given by the client onto the output
	// stream. Returns errors encountered while sending. Blocks if there is
	// no buffer space.
	Send([]byte) error

	// Close indicates that no more bytes will be sent.
	Close() error
}

// BlobReader is an interface for getting a blob.
type BlobReader interface {
	// Advance() stages bytes so that they may be retrieved via
	// Value(). Returns true iff there are bytes to retrieve. Advance() must
	// be called before Value() is called. The caller is expected to read
	// until Advance() returns false, or to call Cancel().
	Advance() bool

	// Value() returns the bytes that were staged by Advance(). May panic if
	// Advance() returned false or was not called. Never blocks.
	Value() []byte

	// Err() returns any error encountered by Advance. Never blocks.
	Err() error

	// Cancel notifies the stream provider that it can stop producing
	// elements.  The client must call Cancel if it does not iterate through
	// all elements (i.e. until Advance returns false). Cancel is idempotent
	// and can be called concurrently with a goroutine that is iterating via
	// Advance.  Cancel causes Advance to subsequently return false. Cancel
	// does not block.
	Cancel()
}

// BlobStatus is an interface for getting the status of a blob transfer.
type BlobStatus interface {
	// Advance() stages an item so that it may be retrieved via
	// Value(). Returns true iff there are items to retrieve. Advance() must
	// be called before Value() is called. The caller is expected to read
	// until Advance() returns false, or to call Cancel().
	Advance() bool

	// Value() returns the item that was staged by Advance(). May panic if
	// Advance() returned false or was not called. Never blocks.
	Value() wire.BlobFetchStatus

	// Err() returns any error encountered by Advance. Never blocks.
	Err() error

	// Cancel notifies the stream provider that it can stop producing
	// elements.  The client must call Cancel if it does not iterate through
	// all elements (i.e. until Advance returns false). Cancel is idempotent
	// and can be called concurrently with a goroutine that is iterating via
	// Advance.  Cancel causes Advance to subsequently return false. Cancel
	// does not block.
	Cancel()
}

// Each database has a Schema associated with it which defines the current
// version of the database. When a new version of app wishes to change
// its data in a way that it is not compatible with the old app's data,
// the app must change the schema version and perform relevant upgrade logic.
// The conflict resolution rules are also associated with the
// schema version. Hence if the conflict resolution rules change then the schema
// version also must be bumped.
//
// Schema provides metadata and a ConflictResolver for a given database.
// ConflictResolver is purely local and not persisted.
type Schema struct {
	Metadata wire.SchemaMetadata
	Resolver ConflictResolver
}

// ConflictResolver interface allows the app to define resolution of conflicts
// that it requested to handle.
type ConflictResolver interface {
	OnConflict(ctx *context.T, conflict *Conflict) Resolution
}

// Get takes a reference to an instance of a type that is expected to be
// represented by Value.
func (v *Value) Get(value interface{}) error {
	return v.Val.ToValue(value)
}

// NewValue creates a new Value to be added to Resolution.
func NewValue(ctx *context.T, data interface{}) (*Value, error) {
	if data == nil {
		return nil, verror.New(verror.ErrBadArg, ctx, "data cannot be nil")
	}
	rawBytes, err := vom.RawBytesFromValue(data)
	if err != nil {
		return nil, err
	}
	return &Value{
		Val:       rawBytes,
		WriteTs:   time.Now(), // ignored by syncbase
		Selection: wire.ValueSelectionOther,
	}, nil
}

package store

import "v.io/v23/verror"

var (
	// ErrKeyExists means the given key already exists in the store.
	ErrKeyExists = verror.NewID("KeyExists")
	// ErrUnknownKey means the given key does not exist in the store.
	ErrUnknownKey = verror.NewID("UnknownKey")
)

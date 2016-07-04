// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package common

// Constants related to storage engine keys.
// Note, these are persisted and therefore must not be modified.
const (
	AppPrefix             = "a"
	CollectionPermsPrefix = "c"
	DatabasePrefix        = "d"
	DbGCPrefix            = "g"
	DbInfoPrefix          = "i"
	LogPrefix             = "l"
	VClockPrefix          = "q"
	RowPrefix             = "r"
	ServicePrefix         = "s"
	VersionPrefix         = "v"
	SyncPrefix            = "y"

	// KeyPartSep is a separator for parts of storage engine keys, e.g. separating
	// collection id from row key.
	KeyPartSep = "\xfe"

	// PrefixRangeLimitSuffix is a key suffix that indicates the end of a prefix
	// range. Must be greater than any character allowed in client-specified keys.
	PrefixRangeLimitSuffix = "\xff"

	// AppDir is the filesystem directory that holds all app databases.
	AppDir = "apps"

	// DbDir is the filesystem directory that holds all databases for an app.
	DbDir = "dbs"
)

// Constants related to object names.
const (
	// Object name component for Syncbase-to-Syncbase (sync) RPCs.
	// Sync object names have the form:
	//     <syncbase>/%%sync/...
	SyncbaseSuffix = "%%sync"
)

// Other constants.
const (
	// The pool.ntp.org project is a big virtual cluster of timeservers providing
	// reliable easy to use NTP service for millions of clients.
	// For more information, see: http://www.pool.ntp.org/en/
	NtpDefaultHost = "pool.ntp.org:123"
)

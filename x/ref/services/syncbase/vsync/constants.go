// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

import "v.io/x/ref/services/syncbase/common"

// Key prefixes for sync-related metadata.
var (
	dagNodePrefix  = common.JoinKeyParts(common.SyncPrefix, "a")
	dagHeadPrefix  = common.JoinKeyParts(common.SyncPrefix, "b")
	dagBatchPrefix = common.JoinKeyParts(common.SyncPrefix, "c")
	dbssKey        = common.JoinKeyParts(common.SyncPrefix, "d") // database sync state
	sgIdPrefix     = common.JoinKeyParts(common.SyncPrefix, "i") // syncgroup ID --> syncgroup local state
	logPrefix      = common.JoinKeyParts(common.SyncPrefix, "l") // log state
	sgDataPrefix   = common.JoinKeyParts(common.SyncPrefix, "s") // syncgroup (ID, version) --> syncgroup synced state
)

const (
	// The sync log contains <logPrefix>:<logDataPrefix> records (for data) and
	// <logPrefix>:<sgoid> records (for syncgroup metadata), where <logDataPrefix>
	// is defined below, and <sgoid> is <sgDataPrefix>:<GroupId>.
	logDataPrefix = "d"
)

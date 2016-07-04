// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#ifndef V23_SYNCBASE_LIB_H_
#define V23_SYNCBASE_LIB_H_

#include <stdbool.h>
#include <stdint.h>

////////////////////////////////////////
// Generic types

typedef uint8_t v23_syncbase_Bool;

// string
typedef struct {
  char* p;
  int n;
} v23_syncbase_String;

// []byte
typedef struct {
  uint8_t* p;
  int n;
} v23_syncbase_Bytes;

// []string
typedef struct {
  v23_syncbase_String* p;
  int n;
} v23_syncbase_Strings;

////////////////////////////////////////
// Vanadium-specific types

// verror.E
typedef struct {
  v23_syncbase_String id;
  unsigned int actionCode;
  v23_syncbase_String msg;
  v23_syncbase_String stack;
} v23_syncbase_VError;

// access.Permissions
// TODO(sadovsky): Decide how to represent perms.
typedef struct {
  v23_syncbase_Bytes json;
} v23_syncbase_Permissions;

////////////////////////////////////////
// Syncbase-specific types

// syncbase.Id
typedef struct {
  v23_syncbase_String blessing;
  v23_syncbase_String name;
} v23_syncbase_Id;

// []syncbase.Id
typedef struct {
  v23_syncbase_Id* p;
  int n;
} v23_syncbase_Ids;

// syncbase.BatchOptions
typedef struct {
  v23_syncbase_String hint;
  bool readOnly;
} v23_syncbase_BatchOptions;

// syncbase.CollectionRowPattern
typedef struct {
  v23_syncbase_String collectionBlessing;
  v23_syncbase_String collectionName;
  v23_syncbase_String rowKey;
} v23_syncbase_CollectionRowPattern;

// []syncbase.CollectionRowPattern
typedef struct {
  v23_syncbase_CollectionRowPattern* p;
  int n;
} v23_syncbase_CollectionRowPatterns;

// syncbase.ChangeType
typedef enum v23_syncbase_ChangeType {
  v23_syncbase_ChangeTypePut = 0,
  v23_syncbase_ChangeTypeDelete = 1
} v23_syncbase_ChangeType;

// syncbase.EntityType
typedef enum v23_syncbase_EntityType {
  v23_syncbase_EntityTypeRoot = 0,
  v23_syncbase_EntityTypeCollection = 1,
  v23_syncbase_EntityTypeRow = 2
} v23_syncbase_EntityType;

// syncbase.WatchChange
typedef struct {
  v23_syncbase_EntityType entityType;
  v23_syncbase_Id collection;
  v23_syncbase_String row;
  v23_syncbase_ChangeType changeType;
  v23_syncbase_Bytes value;
  v23_syncbase_Bytes resumeMarker;
  bool fromSync;
  bool continued;
} v23_syncbase_WatchChange;

// syncbase.KeyValue
typedef struct {
  v23_syncbase_String key;
  v23_syncbase_Bytes value;
} v23_syncbase_KeyValue;

// syncbase.SyncgroupSpec
typedef struct {
  v23_syncbase_String description;
  v23_syncbase_String publishSyncbaseName;
  v23_syncbase_Permissions perms;
  v23_syncbase_Ids collections;
  v23_syncbase_Strings mountTables;
  bool isPrivate;
} v23_syncbase_SyncgroupSpec;

// syncbase.SyncgroupMemberInfo
typedef struct {
  uint8_t syncPriority;
  uint8_t blobDevType;
} v23_syncbase_SyncgroupMemberInfo;

// map[string]syncbase.SyncgroupMemberInfo
typedef struct {
  v23_syncbase_String* keys;
  v23_syncbase_SyncgroupMemberInfo* values;
  int n;
} v23_syncbase_SyncgroupMemberInfoMap;

// syncbase.discovery.Invite
typedef struct {
  v23_syncbase_Id syncgroup;
  v23_syncbase_Strings addresses;
  v23_syncbase_Strings blessingNames;
} v23_syncbase_Invite;

// syncbase.discovery.AppPeer
typedef struct {
  v23_syncbase_String appName;
  v23_syncbase_String blessings;
  bool isLost;
} v23_syncbase_AppPeer;

////////////////////////////////////////
// Functions

typedef void* v23_syncbase_Handle;

typedef struct {
  v23_syncbase_Handle handle;
  void (*onChange)(v23_syncbase_Handle handle, v23_syncbase_WatchChange);
  void (*onError)(v23_syncbase_Handle handle, v23_syncbase_VError);
} v23_syncbase_DbWatchPatternsCallbacks;

typedef struct {
  v23_syncbase_Handle handle;
  void (*onKeyValue)(v23_syncbase_Handle handle, v23_syncbase_KeyValue);
  void (*onDone)(v23_syncbase_Handle handle, v23_syncbase_VError);
} v23_syncbase_CollectionScanCallbacks;

typedef struct {
  v23_syncbase_Handle handle;
  void (*onInvite)(v23_syncbase_Handle handle, v23_syncbase_Invite);
} v23_syncbase_DbSyncgroupInvitesCallbacks;

typedef struct {
  v23_syncbase_Handle handle;
  void (*onPeer)(v23_syncbase_Handle handle, v23_syncbase_AppPeer);
} v23_syncbase_NeighborhoodScanCallbacks;

#endif  // V23_SYNCBASE_LIB_H_

// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// This file contains helpers to minimize the number of cgo calls, which have
// some overhead.
// Some conventions:
//
// Errors are represented by a null-terminated C string. NULL means no error.
// All operations that can raise an error are passed a "char** errptr" as the
// last argument. *errptr should be NULL.
// On failure, leveldb sets *errptr to a malloc()ed error message.
//
// All of the pointer arguments must be non-NULL.

#ifndef V_IO_SYNCBASE_X_REF_SERVICES_SYNCBASE_STORE_LEVELDB_SYNCBASE_LEVELDB_H_
#define V_IO_SYNCBASE_X_REF_SERVICES_SYNCBASE_STORE_LEVELDB_SYNCBASE_LEVELDB_H_

#ifdef __cplusplus
extern "C" {
#endif

#include "leveldb/c.h"

// Fields of this struct are accessed from go directly without cgo calls.
struct syncbase_leveldb_iterator_t {
  leveldb_iterator_t* rep;
  unsigned char is_valid;
  char const* key;
  size_t key_len;
  char const* val;
  size_t val_len;
};

typedef struct syncbase_leveldb_iterator_t syncbase_leveldb_iterator_t;

// Returns iterator that points to first key that is not less than |start|.
// The returned iterator must be passed to |syncbase_leveldb_iter_destroy|
// when finished.
syncbase_leveldb_iterator_t* syncbase_leveldb_create_iterator(
    leveldb_t* db,
    const leveldb_readoptions_t* options,
    const char* start, size_t start_len);

// Deallocates iterator returned by |syncbase_leveldb_create_iterator|.
void syncbase_leveldb_iter_destroy(syncbase_leveldb_iterator_t*);

// Moves to the next entry in the source. After this call, |is_valid| is
// true iff the iterator was not positioned at the last entry in the source.
// REQUIRES: |is_valid| is true.
void syncbase_leveldb_iter_next(syncbase_leveldb_iterator_t* iter);

// Returns a non-nil error iff the iterator encountered any errors.
void syncbase_leveldb_iter_get_error(
    const syncbase_leveldb_iterator_t* iter, char** errptr);

// Returns approximate filesystem usage.
uint64_t syncbase_leveldb_filesystem_bytes(const leveldb_t* db);

// Returns the number of files at all levels.
uint64_t syncbase_leveldb_file_count(const leveldb_t* db);

#ifdef __cplusplus
}  // end extern "C"
#endif

#endif  // V_IO_SYNCBASE_X_REF_SERVICES_SYNCBASE_STORE_LEVELDB_SYNCBASE_LEVELDB_H_

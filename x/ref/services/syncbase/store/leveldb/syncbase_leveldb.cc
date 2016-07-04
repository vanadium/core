// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file is intended to be C++ so that we can access C++ LevelDB interface
// directly if necessary.

#include "syncbase_leveldb.h"
#include <cstdlib>
#include <cstring>

namespace {

// Strictly speaking this is not the last possible key, because this key with
// a character added would be lexicographically greater.
//
// TODO(eobrain): See if there is a way to specify a limit greater than all
// possible keys.  NULL or "" does not work.
const char kAfterlastKey[] = "\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff";

void PopulateIteratorFields(syncbase_leveldb_iterator_t* iter) {
  iter->is_valid = leveldb_iter_valid(iter->rep);
  if (!iter->is_valid) {
    return;
  }
  iter->key = leveldb_iter_key(iter->rep, &iter->key_len);
  iter->val = leveldb_iter_value(iter->rep, &iter->val_len);
}

// Read a property, expecting it to be an integer. Return the integer or -1 if
// it is not.
int64_t IntegerProperty(const leveldb_t* db, const char* name) {
  char* value = leveldb_property_value(const_cast<leveldb_t*>(db), name);
  if (value == NULL || value[0] == '\0') {  // null or empty string
    return -1;
  }
  // TODO(eobrain): When C++11 is supported replace atoi by std::stol and
  // try-catch.
  int result = atoi(value);
  leveldb_free(value);
  return result;
}

}  // namespace

extern "C" {

syncbase_leveldb_iterator_t* syncbase_leveldb_create_iterator(
    leveldb_t* db,
    const leveldb_readoptions_t* options,
    const char* start, size_t start_len) {
  syncbase_leveldb_iterator_t* result = new syncbase_leveldb_iterator_t;
  result->rep = leveldb_create_iterator(db, options);
  leveldb_iter_seek(result->rep, start, start_len);
  PopulateIteratorFields(result);
  return result;
}

void syncbase_leveldb_iter_destroy(syncbase_leveldb_iterator_t* iter) {
  leveldb_iter_destroy(iter->rep);
  delete iter;
}

void syncbase_leveldb_iter_next(syncbase_leveldb_iterator_t* iter) {
  leveldb_iter_next(iter->rep);
  PopulateIteratorFields(iter);
}

void syncbase_leveldb_iter_get_error(
    const syncbase_leveldb_iterator_t* iter, char** errptr) {
  leveldb_iter_get_error(iter->rep, errptr);
}

uint64_t syncbase_leveldb_filesystem_bytes(const leveldb_t* db) {
  uint64_t result;
  const char* begin[] = {""};
  const char* end[] = {kAfterlastKey};
  size_t begin_len[] = {0};
  size_t end_len[] = {strlen(kAfterlastKey)};
  leveldb_approximate_sizes(const_cast<leveldb_t*>(db), 1, begin, begin_len,
                            end, end_len, &result);
  return result;
}

uint64_t syncbase_leveldb_file_count(const leveldb_t* db) {
  // The '#' will be replaced by '0', '1', ... '9'
  char name[] = "leveldb.num-files-at-level#";
  int digitPosition = strlen(name) - 1;
  uint64_t result = 0;
  for (char ch = '0'; ch <= '9'; ch++) {
    name[digitPosition] = ch;
    int levelCount = IntegerProperty(db, name);
    if (levelCount == -1) {
      break;
    }
    result += levelCount;
  }
  return result;
}

}  // end extern "C"

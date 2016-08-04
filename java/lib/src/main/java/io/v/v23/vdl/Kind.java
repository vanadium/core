// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

public enum Kind {
    ANY,
    OPTIONAL,

    BOOL,
    BYTE,
    UINT16,
    UINT32,
    UINT64,
    INT8,
    INT16,
    INT32,
    INT64,
    FLOAT32,
    FLOAT64,
    STRING,
    ENUM,
    TYPEOBJECT,

    ARRAY,
    LIST,
    SET,
    MAP,
    STRUCT,
    UNION
}

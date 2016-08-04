// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

/**
 * AbstractVdlStruct is a basic class for values of type VDL struct.
 * It is used to get VDL type from classes through reflection.
 */
public abstract class AbstractVdlStruct extends VdlValue {
    private static final long serialVersionUID = 1L;

    public AbstractVdlStruct(VdlType type) {
        super(type);
        assertKind(Kind.STRUCT);
    }
}

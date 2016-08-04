// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

import java.io.Serializable;

/**
 * VdlField represents a struct or union field in a VDL type.
 */
public final class VdlField implements Serializable {
    private static final long serialVersionUID = 1L;

    private final String name;
    private final VdlType type;

    public VdlField(String name, VdlType type) {
        this.name = name;
        this.type = type;
    }

    public String getName() {
        return name;
    }

    public VdlType getType() {
        return type;
    }
}

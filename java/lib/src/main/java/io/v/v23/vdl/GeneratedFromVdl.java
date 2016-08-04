// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

import java.lang.annotation.ElementType;
import java.lang.annotation.Retention;
import java.lang.annotation.RetentionPolicy;
import java.lang.annotation.Target;

/**
 * Stores VDL information about the given entity.
 */
@Retention(RetentionPolicy.RUNTIME)
@Target({ElementType.TYPE, ElementType.FIELD})
public @interface GeneratedFromVdl {
    /**
     * The name of the VDL entity from which this entity was generated.
     */
    String name();
    /**
     * The index of VDL struct or union field or enum as defined in VDL.
     */
    int index() default 0;
}

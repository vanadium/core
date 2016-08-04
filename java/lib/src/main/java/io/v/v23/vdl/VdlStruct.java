// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

import java.util.HashMap;
import java.util.Map;

/**
 * VdlStruct is a map based representation of a VDL struct.
 */
public class VdlStruct extends AbstractVdlStruct {
    private static final long serialVersionUID = 1L;

    private final Map<String, VdlValue> fields;
    private final Map<String, VdlType> fieldTypes;

    public VdlStruct(VdlType type) {
        super(type);
        fields = new HashMap<String, VdlValue>();
        fieldTypes = new HashMap<String, VdlType>();
        for (VdlField structField : type.getFields()) {
            fieldTypes.put(structField.getName(), structField.getType());
        }
    }

    /**
     * Tries to assign a new value for specified field. Assigns value and returns true if the struct
     * has a field with specified name and the new matches the field type, otherwise returns false.
     *
     * @param name name of the field
     * @param value value to assign
     * @return true iff value is successfully assigned
     */
    public boolean assignField(String name, VdlValue value) {
        VdlType expectedType = fieldTypes.get(name);
        if (expectedType == null || !expectedType.equals(value.vdlType())) {
            return false;
        }
        fields.put(name, value);
        return true;
    }

    /**
     * Returns value of field with specified name
     *
     * @param name name of the field
     * @return value of field or null if struct has no such field or value was never assigned
     */
    public VdlValue getField(String name) {
        return fields.get(name);
    }

    @Override
    public boolean equals(Object obj) {
        if (this == obj) return true;
        if (obj == null) return false;
        if (this.getClass() != obj.getClass()) return false;
        VdlStruct other = (VdlStruct) obj;
        return fields.equals(other.fields);
    }

    @Override
    public int hashCode() {
        return fields.hashCode();
    }

    @Override
    public String toString() {
        return fields.toString();
    }
}

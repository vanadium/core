// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vom;

import io.v.v23.vdl.Kind;
import io.v.v23.vdl.Types;
import io.v.v23.vdl.VdlField;
import io.v.v23.vdl.VdlType;

import java.util.HashMap;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;

/**
 * TypeCompatibility provides helpers to check compatibility of VDL types.
 */
public final class TypeCompatibility {
    /**
     * Returns true iff provided types are compatible.
     *
     * Compatibility is symmetric and transitive, except for the special Any type.
     * Here are the rules:
     *   o Any is compatible with all types.
     *   o Optional is ignored for all rules (e.g. ?int is treated as int).
     *   o Bool is only compatible with Bool.
     *   o TypeObject is only compatible with TypeObject.
     *   o Numbers are mutually compatible.
     *   o String, enum, []byte and [N]byte are mutually compatible.
     *   o Array and list are compatible if their elems are compatible.
     *   o Set, map and struct are compatible if all keys K* are compatible, and all fields F* are
     *     compatible:
     *     - map[string]F* is compatible with struct{_ F*; ...}
     *     - set[K*] is compatible with set[K*] and map[K*]bool
     *     - set[string] is compatible with struct{_ bool; ...}
     *     - Two struct types are compatible if all fields with the same name are
     *       compatible, and at least one field has the same name, or one of the
     *       types is an empty struct.
     *   o Two union types are compatible if all fields with the same name are
     *     compatible, and at least one field has the same name.
     */
    public static boolean compatible(VdlType a, VdlType b) {
        Map<VdlType, Set<VdlType>> seen = new HashMap<VdlType, Set<VdlType>>();
        return compatible(a, b, seen);
    }

    /**
     * Returns true iff provided types are compatible or a pair of types (a, b) was already visited.
     * It is OK to return true for visited type pairs as we need to find only one type compatibility
     * mismatch.
     *
     * @param seen the set of visited type pairs
     */
    private static boolean compatible(VdlType a, VdlType b, Map<VdlType, Set<VdlType>> seen) {
        // Remove optional wrapper.
        if (a.getKind() == Kind.OPTIONAL) {
            a = a.getElem();
        }
        if (b.getKind() == Kind.OPTIONAL) {
            b = b.getElem();
        }
        if (a == b) {
            return true;
        }
        // Look-up in seen.
        Set<VdlType> set = seen.get(a);
        if (set == null) {
            set = new HashSet<VdlType>();
            seen.put(a, set);
        }
        if (set.contains(b)) {
            return true;
        }
        set.add(b);
        // Handle Any.
        if (a.getKind() == Kind.ANY || b.getKind() == Kind.ANY) {
            return true;
        }
        // Handle simple scalar VDL types.
        if (isNumber(a)) {
            return isNumber(b);
        } else if (a.getKind() == Kind.BOOL) {
            return b.getKind() == Kind.BOOL;
        } else if (a.getKind() == Kind.TYPEOBJECT) {
            return b.getKind() == Kind.TYPEOBJECT;
        }
        // We must check if either a or b is []byte and handle it here first, to
        // ensure it doesn't fall through to the standard array/list handling.  This
        // ensures that []byte isn't compatible with []uint16 and other lists or
        // arrays of numbers.
        boolean aIsBytes = isStringEnumBytes(a), bIsBytes = isStringEnumBytes(b);
        if (aIsBytes|| bIsBytes) {
            return aIsBytes && bIsBytes;
        }
        // Handle composite VDL.
        switch (a.getKind()) {
            case ARRAY:
            case LIST:
                if (b.getKind() == Kind.ARRAY || b.getKind() == Kind.LIST) {
                    return compatible(a.getElem(), b.getElem(), seen);
                }
                return false;
            case MAP:
            case SET:
                switch (b.getKind()) {
                    case MAP:
                    case SET:
                        return mapsCompatible(a, b, seen);
                    case STRUCT:
                        return structAndMapCompatible(b, a, seen);
                    default:
                        return false;
                }
            case STRUCT:
                switch (b.getKind()) {
                    case MAP:
                    case SET:
                        return structAndMapCompatible(a, b, seen);
                    case STRUCT:
                        if (isEmptyStruct(a) || isEmptyStruct(b)) {
                            return true;
                        }
                        return fieldsCompatible(a, b, seen);
                    default:
                        return false;
                }
            case UNION:
                if (b.getKind() == Kind.UNION) {
                    return fieldsCompatible(a, b, seen);
                }
                return false;
            default:
                throw new IllegalArgumentException("Unsupported VDL type " + a);
        }
    }

    private static boolean isNumber(VdlType type) {
        switch (type.getKind()) {
            case BYTE:
            case FLOAT32:
            case FLOAT64:
            case INT16:
            case INT32:
            case INT64:
            case UINT16:
            case UINT32:
            case UINT64:
                return true;
            default:
                return false;
        }
    }

    private static boolean isStringEnumBytes(VdlType type) {
        return type.getKind() == Kind.STRING || type.getKind() == Kind.ENUM
                || BinaryUtil.isBytes(type);
    }

    private static boolean isEmptyStruct(VdlType type) {
        return type.getKind() == Kind.STRUCT && type.getFields().isEmpty();
    }

    private static boolean mapsCompatible(VdlType a, VdlType b, Map<VdlType, Set<VdlType>> seen) {
        if (!compatible(a.getKey(), b.getKey(), seen)) {
            return false;
        }
        VdlType aElem = a.getKind() == Kind.MAP ? a.getElem() : Types.BOOL;
        VdlType bElem = b.getKind() == Kind.MAP ? b.getElem() : Types.BOOL;
        return compatible(aElem, bElem, seen);
    }

    private static boolean structAndMapCompatible(VdlType struct, VdlType map,
            Map<VdlType, Set<VdlType>> seen) {
        if (isEmptyStruct(struct) || !compatible(Types.STRING, map.getKey(), seen)) {
            return false;
        }
        VdlType elem = map.getKind() == Kind.MAP ? map.getElem() : Types.BOOL;
        for (VdlField field : struct.getFields()) {
            if (!compatible(elem, field.getType(), seen)) {
                return false;
            }
        }
        return true;
    }

    private static boolean fieldsCompatible(VdlType a, VdlType b, Map<VdlType, Set<VdlType>> seen) {
        if (a.getFields().size() > b.getFields().size()) {
            return fieldsCompatible(b, a, seen);
        }
        Map<String, VdlType> aFields = new HashMap<String, VdlType>();
        for (VdlField field : a.getFields()) {
            aFields.put(field.getName(), field.getType());
        }
        boolean fieldMatched = false;
        for (VdlField field : b.getFields()) {
            VdlType type = aFields.get(field.getName());
            if (type != null) {
                if (!compatible(type, field.getType())) {
                    return false;
                }
                fieldMatched = true;
            }
        }
        return fieldMatched;
    }
}

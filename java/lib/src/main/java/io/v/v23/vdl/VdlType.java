// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

import com.google.common.base.Joiner;
import com.google.common.base.Strings;
import com.google.common.collect.ImmutableList;

import java.io.Serializable;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

/**
 * Type represents VDL types.
 */
public final class VdlType implements Serializable {
    private static final long serialVersionUID = 1L;

    private Kind kind; // used by all kinds
    private String name; // used by all kinds
    private ImmutableList<String> labels; // used by enum
    private int length; // used by array
    private VdlType key; // used by set, map
    private VdlType elem; // used by array, list, map, optional
    private ImmutableList<VdlField> fields; // used by struct and union
    private String typeString; // used by all kinds, filled in by getUniqueType

    /**
     * Stores a mapping from type string to VDL type instance. This is used to make sure that
     * vdlTypeA.typeString == vdlTypeB.typeString => vdlTypeA == vdlTypeB.
     */
    private static final Map<String, VdlType> uniqueTypes =
            new ConcurrentHashMap<String, VdlType>();

    /**
     * Generated a type string representing type, which also is its human-readable representation.
     * To break loops we cache named types only, as you can't define an unnamed cyclic type in VDL.
     * We also ensure that there is at most one VDL type instance for each name. These two
     * assumptions make VDL type graph isomorphism check based on type strings straightforward.
     */
    private static String typeString(VdlType type, final Map<String, VdlType> seen) {
        if (!Strings.isNullOrEmpty(type.name)) {
            VdlType seenType = seen.get(type.name);
            if (seenType != null) {
                if (seenType != type) {
                    throw new IllegalArgumentException("Duplicate type name " + type.name);
                }
                return type.name;
            }
            seen.put(type.name, type);
        }
        String result = "";
        if (!Strings.isNullOrEmpty(type.name)) {
            result = type.name + " ";
        }
        switch (type.kind) {
            case ENUM:
                return result + "enum{" + Joiner.on(";").join(type.labels) + "}";
            case ARRAY:
                return result + "[" + type.length + "]" + typeString(type.elem, seen);
            case LIST:
                return result + "[]" + typeString(type.elem, seen);
            case SET:
                return result + "set[" + typeString(type.key, seen) + "]";
            case MAP:
                return result + "map[" + typeString(type.key, seen) + "]"
                        + typeString(type.elem, seen);
            case STRUCT:
            case UNION:
                if (type.kind == Kind.STRUCT) {
                    result += "struct{";
                } else {
                    result += "union{";
                }
                for (int i = 0; i < type.fields.size(); i++) {
                    if (i > 0) {
                        result += ";";
                    }
                    VdlField field = type.fields.get(i);
                    result += field.getName() + " " + typeString(field.getType(), seen);
                }
                return result + "}";
            case OPTIONAL:
                return result + "?" + typeString(type.elem, seen);
            default:
                return result + type.kind.name().toLowerCase();
            }
    }

    private Object readResolve() {
        return getUniqueType(this);
    }

    private static synchronized VdlType getUniqueType(VdlType type) {
        if (type == null) {
            return null;
        }
        if (type.typeString == null) {
            type.typeString = typeString(type, new HashMap<String, VdlType>());
        }
        VdlType uniqueType = uniqueTypes.get(type.typeString);
        if (uniqueType != null) {
            return uniqueType;
        }
        uniqueTypes.put(type.typeString, type);
        type.key = getUniqueType(type.key);
        type.elem = getUniqueType(type.elem);
        if (type.fields != null) {
            ImmutableList.Builder<VdlField> builder = new ImmutableList.Builder<VdlField>();
            for (VdlField field : type.fields) {
                builder.add(new VdlField(field.getName(), getUniqueType(field.getType())));
            }
            type.fields = builder.build();
        }
        return type;
    }

    private VdlType() {}

    public Kind getKind() {
        return kind;
    }

    public String getName() {
        return name;
    }

    public List<String> getLabels() {
        return labels;
    }

    public int getLength() {
        return length;
    }

    public VdlType getKey() {
        return key;
    }

    public VdlType getElem() {
        return elem;
    }

    public List<VdlField> getFields() {
        return fields;
    }

    @Override
    public String toString() {
        return typeString;
    }

    /**
     * Builder builds Types. There are two phases: 1) Create PendingType objects and describe each
     * type, and 2) call build(). When build() is called, all types are created and may be retrieved
     * by calling built() on the pending type. This two-phase building enables support for recursive
     * types, and also makes it easy to construct a group of dependent types without determining
     * their dependency ordering.
     */
    public static final class Builder {
        private final List<PendingType> pendingTypes;

        public Builder() {
            pendingTypes = new ArrayList<PendingType>();
        }

        public PendingType newPending() {
            PendingType type = new PendingType();
            pendingTypes.add(type);
            return type;
        }

        public PendingType newPending(Kind kind) {
            return newPending().setKind(kind);
        }

        public PendingType listOf(PendingType elem) {
            return newPending(Kind.LIST).setElem(elem);
        }

        public PendingType setOf(PendingType key) {
            return newPending(Kind.SET).setKey(key);
        }

        public PendingType mapOf(PendingType key, PendingType elem) {
            return newPending(Kind.MAP).setKey(key).setElem(elem);
        }

        public PendingType optionalOf(PendingType elem) {
            return newPending(Kind.OPTIONAL).setElem(elem);
        }

        public PendingType builtPendingFromType(VdlType vdlType) {
            return new PendingType(vdlType);
        }

        public void build() {
            for (PendingType type : pendingTypes) {
                type.prepare();
            }
            for (PendingType type : pendingTypes) {
                type.build();
            }
            pendingTypes.clear();
        }
    }

    public static final class PendingType {
        private VdlType vdlType;
        private final List<String> labels;
        private final List<VdlField> fields;
        private boolean built;

        private PendingType(VdlType vdlType) {
            this.vdlType = vdlType;
            labels = null;
            fields = null;
            built = true;
        }

        private PendingType() {
            vdlType = new VdlType();
            labels = new ArrayList<String>();
            fields = new ArrayList<VdlField>();
            built = false;
        }

        private void prepare() {
            if (built) {
                return;
            }
            switch (vdlType.kind) {
                case ENUM:
                    vdlType.labels = ImmutableList.copyOf(labels);
                    break;
                case STRUCT:
                case UNION:
                    vdlType.fields = ImmutableList.copyOf(fields);
                    break;
                default:
                    // do nothing
            }
        }

        private void build() {
            if (built) {
                return;
            }
            vdlType = getUniqueType(vdlType);
            built = true;
        }

        public PendingType setKind(Kind kind) {
            assertNotBuilt();
            vdlType.kind = kind;
            return this;
        }

        public PendingType setName(String name) {
            assertNotBuilt();
            vdlType.name = name;
            return this;
        }

        public PendingType addLabel(String label) {
            assertNotBuilt();
            assertOneOfKind(Kind.ENUM);
            labels.add(label);
            return this;
        }

        public PendingType setLength(int length) {
            assertNotBuilt();
            assertOneOfKind(Kind.ARRAY);
            vdlType.length = length;
            return this;
        }

        public PendingType setKey(VdlType key) {
            assertNotBuilt();
            assertOneOfKind(Kind.SET, Kind.MAP);
            vdlType.key = key;
            return this;
        }

        public PendingType setKey(PendingType key) {
            return setKey(key.vdlType);
        }

        public PendingType setElem(VdlType elem) {
            assertNotBuilt();
            assertOneOfKind(Kind.ARRAY, Kind.LIST, Kind.MAP, Kind.OPTIONAL);
            vdlType.elem = elem;
            return this;
        }

        public PendingType setElem(PendingType elem) {
            return setElem(elem.vdlType);
        }

        public PendingType addField(String name, VdlType type) {
            assertNotBuilt();
            assertOneOfKind(Kind.STRUCT, Kind.UNION);
            fields.add(new VdlField(name, type));
            return this;
        }

        public PendingType addField(String name, PendingType type) {
            return addField(name, type.vdlType);
        }

        public PendingType assignBase(VdlType type) {
            assertNotBuilt();
            this.vdlType.kind = type.kind;
            this.vdlType.length = type.length;
            this.vdlType.key = type.key;
            this.vdlType.elem = type.elem;
            labels.clear();
            if (type.labels != null) {
                labels.addAll(type.labels);
            }
            fields.clear();
            if (type.fields != null) {
                fields.addAll(type.fields);
            }
            return this;
        }

        public PendingType assignBase(PendingType pending) {
            return assignBase(pending.vdlType);
        }

        public VdlType built() {
            if (!built) {
                throw new IllegalStateException("The pending type is not built yet");
            }
            return vdlType;
        }

        private void assertNotBuilt() {
            if (built) {
                throw new IllegalStateException("The pending type is already built");
            }
        }

        private void assertOneOfKind(Kind... kinds) {
            for (Kind kind : kinds) {
                if (vdlType.kind == kind) {
                    return;
                }
            }
            throw new IllegalArgumentException("Unsupported operation for kind " + vdlType.kind);
        }
    }
}

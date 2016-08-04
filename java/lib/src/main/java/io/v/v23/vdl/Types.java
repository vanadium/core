// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

import com.google.common.base.Joiner;
import com.google.common.base.Strings;
import io.v.v23.security.BlessingPattern;
import io.v.v23.security.BlessingPatternNativeConverter;
import io.v.v23.security.Blessings;
import io.v.v23.security.BlessingsNativeConverter;
import io.v.v23.security.Discharge;
import io.v.v23.security.DischargeNativeConverter;
import io.v.v23.security.access.AccessList;
import io.v.v23.security.access.AccessListNativeConverter;
import io.v.v23.vdl.NativeTime.DateTimeConverter;
import io.v.v23.vdl.NativeTime.DurationConverter;
import io.v.v23.vdl.NativeTypes.Converter;
import io.v.v23.vdl.VdlType.Builder;
import io.v.v23.vdl.VdlType.PendingType;
import io.v.v23.verror.VException;
import io.v.v23.verror.VExceptionVdlConverter;

import java.lang.reflect.Field;
import java.lang.reflect.GenericArrayType;
import java.lang.reflect.Modifier;
import java.lang.reflect.ParameterizedType;
import java.lang.reflect.Type;
import java.util.Arrays;
import java.util.Collections;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeMap;
import java.util.concurrent.ConcurrentHashMap;

/**
 * Types provides helpers to create VDL types.
 */
public final class Types {
    /**
     * The {@code VdlType} object representing the VDL type any, it is unnamed.
     */
    public static final VdlType ANY = createPrimitiveType(Kind.ANY);

    /**
     * The {@code VdlType} object representing the VDL type bool, it is unnamed.
     */
    public static final VdlType BOOL = createPrimitiveType(Kind.BOOL);

    /**
     * The {@code VdlType} object representing the VDL type byte, it is unnamed.
     */
    public static final VdlType BYTE = createPrimitiveType(Kind.BYTE);

    /**
     * The {@code VdlType} object representing the VDL type uint16, it is unnamed.
     */
    public static final VdlType UINT16 = createPrimitiveType(Kind.UINT16);

    /**
     * The {@code VdlType} object representing the VDL type uint32, it is unnamed.
     */
    public static final VdlType UINT32 = createPrimitiveType(Kind.UINT32);

    /**
     * The {@code VdlType} object representing the VDL type uint64, it is unnamed.
     */
    public static final VdlType UINT64 = createPrimitiveType(Kind.UINT64);

    /**
     * The {@code VdlType} object representing the VDL type int8, it is unnamed.
     */
    public static final VdlType INT8 = createPrimitiveType(Kind.INT8);

    /**
     * The {@code VdlType} object representing the VDL type int16, it is unnamed.
     */
    public static final VdlType INT16 = createPrimitiveType(Kind.INT16);

    /**
     * The {@code VdlType} object representing the VDL type int32, it is unnamed.
     */
    public static final VdlType INT32 = createPrimitiveType(Kind.INT32);

    /**
     * The {@code VdlType} object representing the VDL type int64, it is unnamed.
     */
    public static final VdlType INT64 = createPrimitiveType(Kind.INT64);

    /**
     * The {@code VdlType} object representing the VDL type float32, it is unnamed.
     */
    public static final VdlType FLOAT32 = createPrimitiveType(Kind.FLOAT32);

    /**
     * The {@code VdlType} object representing the VDL type float64, it is unnamed.
     */
    public static final VdlType FLOAT64 = createPrimitiveType(Kind.FLOAT64);

    /**
     * The {@code VdlType} object representing the VDL type string, it is unnamed.
     */
    public static final VdlType STRING = createPrimitiveType(Kind.STRING);

    /**
     * The {@code VdlType} object representing the VDL type typeObject, it is unnamed.
     */
    public static final VdlType TYPEOBJECT = createPrimitiveType(Kind.TYPEOBJECT);

    private static final Map<Type, VdlType> typeCache = new ConcurrentHashMap<Type, VdlType>();
    private static final Map<VdlType, Type> typeRegistry = new ConcurrentHashMap<VdlType, Type>();
    private static final Map<Type, Converter> nativeTypeRegistry =
            new ConcurrentHashMap<Type, Converter>();

    static {
        typeCache.put(VdlAny.class, ANY);
        typeCache.put(VdlBool.class, BOOL);
        typeCache.put(VdlByte.class, BYTE);
        typeCache.put(VdlUint16.class, UINT16);
        typeCache.put(VdlUint32.class, UINT32);
        typeCache.put(VdlUint64.class, UINT64);
        typeCache.put(VdlInt8.class, INT8);
        typeCache.put(VdlInt16.class, INT16);
        typeCache.put(VdlInt32.class, INT32);
        typeCache.put(VdlInt64.class, INT64);
        typeCache.put(VdlFloat32.class, FLOAT32);
        typeCache.put(VdlFloat64.class, FLOAT64);
        typeCache.put(VdlString.class, STRING);
        typeCache.put(VdlTypeObject.class, TYPEOBJECT);

        typeCache.put(Boolean.TYPE, BOOL);
        typeCache.put(Boolean.class, BOOL);
        typeCache.put(Byte.TYPE, BYTE);
        typeCache.put(Byte.class, BYTE);
        typeCache.put(Short.TYPE, INT16);
        typeCache.put(Short.class, INT16);
        typeCache.put(Integer.TYPE, INT32);
        typeCache.put(Integer.class, INT32);
        typeCache.put(Long.TYPE, INT64);
        typeCache.put(Long.class, INT64);
        typeCache.put(Float.TYPE, FLOAT32);
        typeCache.put(Float.class, FLOAT32);
        typeCache.put(Double.TYPE, FLOAT64);
        typeCache.put(Double.class, FLOAT64);
        typeCache.put(String.class, STRING);

        // When registering native types, make sure to register "child" types first. For example,
        // if VDL type A contains VDL type B and both have native types that you want to register
        // here, you must register A before B.
        registerNativeType(VException.class, VExceptionVdlConverter.INSTANCE);
        registerNativeType(org.joda.time.DateTime.class, DateTimeConverter.INSTANCE);
        registerNativeType(org.joda.time.Duration.class, DurationConverter.INSTANCE);
        registerNativeType(Discharge.class, DischargeNativeConverter.INSTANCE);
        registerNativeType(Blessings.class, BlessingsNativeConverter.INSTANCE);
        registerNativeType(BlessingPattern.class, BlessingPatternNativeConverter.INSTANCE);
        registerNativeType(AccessList.class, AccessListNativeConverter.INSTANCE);
    }

    private static void registerNativeType(Type nativeType, Converter converter) {
        VdlType vdlType = getVdlTypeFromReflect(converter.getWireType());
        typeCache.put(nativeType, vdlType);
        typeRegistry.put(vdlType, nativeType);
        nativeTypeRegistry.put(nativeType, converter);
    }

    private static VdlType createPrimitiveType(Kind kind) {
        Builder builder = new Builder();
        PendingType pending = builder.newPending(kind);
        builder.build();
        return pending.built();
    }

    /**
     * Returns a {@code VdlType} object representing a VDL type of specified kind.
     */
    public static VdlType primitiveTypeFromKind(Kind kind) {
        switch (kind) {
            case ANY:
                return ANY;
            case BOOL:
                return BOOL;
            case BYTE:
                return BYTE;
            case UINT16:
                return UINT16;
            case UINT32:
                return UINT32;
            case UINT64:
                return UINT64;
            case INT8:
                return INT8;
            case INT16:
                return INT16;
            case INT32:
                return INT32;
            case INT64:
                return INT64;
            case FLOAT32:
                return FLOAT32;
            case FLOAT64:
                return FLOAT64;
            case STRING:
                return STRING;
            case TYPEOBJECT:
                return TYPEOBJECT;
            default:
                throw new RuntimeException("Unknown primitive kind " + kind);
        }
    }

    /**
     * A helper used to create a single VDL enum type.
     */
    public static VdlType enumOf(String... labels) {
        Builder builder = new Builder();
        PendingType pending = builder.newPending(Kind.ENUM);
        for (String label : labels) {
            pending.addLabel(label);
        }
        builder.build();
        return pending.built();
    }

    /**
     * A helper used to create a single VDL fixed length array type.
     */
    public static VdlType arrayOf(int len, VdlType elem) {
        Builder builder = new Builder();
        PendingType pending = builder.newPending(Kind.ARRAY).setLength(len).setElem(elem);
        builder.build();
        return pending.built();
    }

    /**
     * A helper used to create a single VDL list type.
     */
    public static VdlType listOf(VdlType elem) {
        Builder builder = new Builder();
        PendingType pending = builder.newPending(Kind.LIST).setElem(elem);
        builder.build();
        return pending.built();
    }

    /**
     * A helper used to create a single VDL set type.
     */
    public static VdlType setOf(VdlType key) {
        Builder builder = new Builder();
        PendingType pending = builder.newPending(Kind.SET).setKey(key);
        builder.build();
        return pending.built();
    }

    /**
     * A helper used to create a single VDL map type.
     */
    public static VdlType mapOf(VdlType key, VdlType elem) {
        Builder builder = new Builder();
        PendingType pending = builder.newPending(Kind.MAP).setKey(key).setElem(elem);
        builder.build();
        return pending.built();
    }

    /**
     * A helper used to create a single VDL struct type.
     */
    public static VdlType structOf(VdlField... fields) {
        Builder builder = new Builder();
        PendingType pending = builder.newPending(Kind.STRUCT);
        for (VdlField field : fields) {
            pending.addField(field.getName(), field.getType());
        }
        builder.build();
        return pending.built();
    }

    /**
     * A helper used to create a single VDL union type.
     */
    public static VdlType unionOf(VdlField... fields) {
        Builder builder = new Builder();
        PendingType pending = builder.newPending(Kind.UNION);
        for (VdlField field : fields) {
            pending.addField(field.getName(), field.getType());
        }
        builder.build();
        return pending.built();
    }

    /**
     * A helper used to create a single VDL optional type.
     */
    public static VdlType optionalOf(VdlType elem) {
        Builder builder = new Builder();
        PendingType pending = builder.newPending(Kind.OPTIONAL).setElem(elem);
        builder.build();
        return pending.built();
    }

    /**
     * A helper used to create a single named VDL type based on another VDL type.
     */
    public static VdlType named(String name, VdlType base) {
        Builder builder = new Builder();
        PendingType pending = builder.newPending().assignBase(base).setName(name);
        builder.build();
        return pending.built();
    }

    /**
     * Returns a {@code NativeTypes.Converter} object for a provided java native type or null
     * if there is no converter from provided java type to its VDL wire representation.
     */
    public static NativeTypes.Converter getNativeTypeConverter(Type type) {
        if (type instanceof Class && VException.class.isAssignableFrom((Class) type)) {
            // We use the converter for VException to convert all of its subclasses as well.
            type = VException.class;
        }
        return nativeTypeRegistry.get(type);
    }

    /**
     * Creates a {@code VdlType} object corresponding to a {@code java.lang.reflect.Type} object.
     * Resolves maps, sets, lists, arrays, primitives and classes generated from *.vdl files.
     * All results are statically cached. All named VDL types are also registered so that the
     * corresponding {@code Type} object can be retrieved by calling {@code getReflectTypeForVdl}.
     *
     * @throws IllegalArgumentException if the VDL type can't be constructed
     */
    public static VdlType getVdlTypeFromReflect(Type type) {
        if (typeCache.containsKey(type)) {
            return typeCache.get(type);
        }
        return synchronizedLookupOrBuildType(type);
    }

    /**
     * Returns a {@code Type} object corresponding to VDL type.
     * We look up named types that were built by calling {@code getVdlTypeFromReflect}, and build
     * the unnamed types that have java native equivalent (all except array, enum, struct, union).
     *
     * @param vdlType the VDL type
     * @return the {@code Type} object
     * @throws IllegalArgumentException if the type can't be constructed
     */
    public static Type getReflectTypeForVdl(VdlType vdlType) {
        Type type = typeRegistry.get(vdlType);
        if (type != null) {
            return type;
        }
        if (!Strings.isNullOrEmpty(vdlType.getName())) {  // named type
            throw new IllegalArgumentException("Can't build java type for VDL type " + vdlType + " - named type is unregistered");
        }

        Type key, elem;
        switch (vdlType.getKind()) {
            case ARRAY:
            case ENUM:
            case STRUCT:
            case UNION:
                throw new IllegalArgumentException("Can't build java type for VDL type " + vdlType + " - illegal unnamed union");
            case ANY:
                return VdlAny.class;
            case BOOL:
                return Boolean.class;
            case BYTE:
                return Byte.class;
            case FLOAT32:
                return Float.class;
            case FLOAT64:
                return Double.class;
            case INT8:
                return VdlInt8.class;
            case INT16:
                return Short.class;
            case INT32:
                return Integer.class;
            case INT64:
                return Long.class;
            case LIST:
                if (vdlType.getElem().getKind() == Kind.BYTE) {
                  return byte[].class;
                }
                elem = getReflectTypeForVdl(vdlType.getElem());
                if (elem != null) {
                    return new ParameterizedTypeImpl(VdlList.class, elem);
                }
                throw new IllegalArgumentException("Can't build java type for VDL type " + vdlType + " - unknown list elem type");
            case MAP:
                key = getReflectTypeForVdl(vdlType.getKey());
                elem = getReflectTypeForVdl(vdlType.getElem());
                if (key != null && elem != null) {
                    return new ParameterizedTypeImpl(VdlMap.class, key, elem);
                }
                throw new IllegalArgumentException("Can't build java type for VDL type " + vdlType + " - unknown map key or elem type");
            case OPTIONAL:
                elem = getReflectTypeForVdl(vdlType.getElem());
                if (elem != null) {
                    return new ParameterizedTypeImpl(VdlOptional.class, elem);
                }
                throw new IllegalArgumentException("Can't build java type for VDL type " + vdlType + " - unkown optional elem type");
            case SET:
                key = getReflectTypeForVdl(vdlType.getKey());
                if (key != null) {
                    return new ParameterizedTypeImpl(VdlSet.class, key);
                }
                throw new IllegalArgumentException("Can't build java type for VDL type " + vdlType + " - unknown set key type");
            case STRING:
                return String.class;
            case TYPEOBJECT:
                return VdlTypeObject.class;
            case UINT16:
                return VdlUint16.class;
            case UINT32:
                return VdlUint32.class;
            case UINT64:
                return VdlUint64.class;
            default:
                throw new IllegalArgumentException("Unsupported VDL type: " + vdlType);
        }
    }

    /**
     * Tries to load a Java class that was generated from named VDL type.
     *
     * @param name the name of VDL type
     * @return loaded class if the class was found, or {@code null} otherwise
     */
    public static Class<?> loadClassForVdlName(String name) {
        String[] parts = name.split("/");
        for (int i = 0; i < parts.length - 1; i++) {
            List<String> subparts = Arrays.asList(parts[i].split("\\."));
            Collections.reverse(subparts);
            parts[i] = Joiner.on(".").join(subparts);
        }
        String className = Joiner.on(".").join(parts);
        try {
            // Lookup and load class.
            return Class.forName(className);
        } catch (ClassNotFoundException | NoClassDefFoundError e) {
            return null;
        }
    }

    private static synchronized VdlType synchronizedLookupOrBuildType(Type type) {
        if (typeCache.containsKey(type)) {
            return typeCache.get(type);
        }
        ReflectToVdlTypeBuilder builder = new ReflectToVdlTypeBuilder();
        PendingType pendingType = builder.lookupOrBuildPending(type);
        builder.buildAndCache();
        return pendingType.built();
    }

    /**
     * Builds VdlType from {@code java.lang.reflect.Type}. All results are cached in typeCahce.
     */
    private static final class ReflectToVdlTypeBuilder {
        private final Builder builder;
        private final Map<Type, PendingType> pendingTypes;

        public ReflectToVdlTypeBuilder() {
            builder = new Builder();
            pendingTypes = new HashMap<Type, PendingType>();
        }

        public void buildAndCache() {
            builder.build();
            for (Map.Entry<Type, PendingType> entry : pendingTypes.entrySet()) {
                Type reflectType = entry.getKey();
                VdlType vdlType = entry.getValue().built();
                typeCache.put(reflectType, vdlType);
                if (!Strings.isNullOrEmpty(vdlType.getName())) {
                    typeRegistry.put(vdlType, reflectType);
                }
            }
        }

        public PendingType lookupOrBuildPending(Type type) {
            PendingType vdlType = lookupType(type);
            if (vdlType != null) {
                return vdlType;
            }
            return buildPendingFromType(type);
        }

        private PendingType lookupType(Type type) {
            if (typeCache.containsKey(type)) {
                return builder.builtPendingFromType(typeCache.get(type));
            }
            if (pendingTypes.containsKey(type)) {
                return pendingTypes.get(type);
            }
            return null;
        }

        private PendingType buildPendingFromType(Type type) {
            Class<?> klass;
            Type[] elementTypes;
            if (type instanceof Class) {
                klass = (Class<?>) type;
                return buildPendingFromClass(klass);
            } else if (type instanceof ParameterizedType) {
                klass = (Class<?>) ((ParameterizedType) type).getRawType();
                elementTypes = ((ParameterizedType) type).getActualTypeArguments();
            } else if (type instanceof GenericArrayType) {
                klass = List.class;
                elementTypes = new Type[1];
                elementTypes[0] = (((GenericArrayType) type).getGenericComponentType());
            } else {
                throw new IllegalArgumentException("Unable to create VDL Type for type " + type);
            }

            PendingType pending;
            if (List.class.isAssignableFrom(klass)) {
                pending = builder.listOf(lookupOrBuildPending(elementTypes[0]));
            } else if (Set.class.isAssignableFrom(klass)) {
                pending = builder.setOf(lookupOrBuildPending(elementTypes[0]));
            } else if (Map.class.isAssignableFrom(klass)) {
                pending = builder.mapOf(lookupOrBuildPending(elementTypes[0]),
                        lookupOrBuildPending(elementTypes[1]));
            } else if (VdlOptional.class.isAssignableFrom(klass)) {
                pending = builder.optionalOf(lookupOrBuildPending(elementTypes[0]));
            } else {
                throw new IllegalArgumentException("Unable to create VDL Type for type " + type);
            }
            pendingTypes.put(type, pending);
            return pending;
        }


        private PendingType buildPendingFromClass(Class<?> klass) {
            PendingType pending;
            if (klass.isArray()) {
                pending = builder.listOf(lookupOrBuildPending(klass.getComponentType()));
                pendingTypes.put(klass, pending);
                return pending;
            }
            if (klass.isAssignableFrom(List.class)) {
                throw new IllegalArgumentException("Unable to create a VDL type from List.class." +
                        "  Consider creating a type using a TypeToken.");
            } else if (klass.isAssignableFrom(Set.class)) {
                throw new IllegalArgumentException("Unable to create a VDL type from Set.class." +
                        "  Consider creating a type using a TypeToken.");
            } else if (klass.isAssignableFrom(Map.class)) {
                throw new IllegalArgumentException("Unable to create a VDL type from Map.class." +
                        "  Consider creating a type using a TypeToken.");
            }
            pending = builder.newPending();
            pendingTypes.put(klass, pending);
            Class<?> superClass = klass.getSuperclass();
            if (superClass == VdlEnum.class) {
                populateEnum(pending, klass);
            } else if (superClass == AbstractVdlStruct.class) {
                if (klass == VdlStruct.class) {
                    throw new IllegalArgumentException("Unable to create VDL Type for " + klass);
                }
                populateStruct(pending, klass);
            } else if (superClass == VdlUnion.class) {
                populateUnion(pending, klass);
            } else if (superClass == VdlArray.class) {
                populateArray(pending, klass);
            } else if (superClass != null && superClass != Object.class) {
                pending.assignBase(lookupOrBuildPending(klass.getGenericSuperclass()));
            } else {
                // Attempt to decode as a struct.
                populateStruct(pending, klass);
            }
            GeneratedFromVdl annotation = klass.getAnnotation(GeneratedFromVdl.class);
            if (annotation != null) {
                pending.setName(annotation.name());
            } else if (klass.getCanonicalName() != null){
                pending.setName(klass.getCanonicalName());
            }
            return pending;
        }

        private void populateEnum(PendingType pending, Class<?> klass) {
            pending.setKind(Kind.ENUM);
            TreeMap<Integer, String> labels = new TreeMap<Integer, String>();
            for (Field field : klass.getDeclaredFields()) {
                GeneratedFromVdl annotation = field.getAnnotation(GeneratedFromVdl.class);
                if (annotation != null) {
                    labels.put(annotation.index(), annotation.name());
                }
            }
            for (Map.Entry<Integer, String> entry : labels.entrySet()) {
                pending.addLabel(entry.getValue());
            }
        }

        private void populateStruct(PendingType pending, Class<?> klass) {
            pending.setKind(Kind.STRUCT);
            TreeMap<Integer, PendingVdlField> fields = new TreeMap<Integer, PendingVdlField>();
            // See if the struct has any annotations.  If not, we assume user has provided
            // a raw class and we try to guess what the annotations would be.
            boolean hasFieldAnnotations = false;
            for (Field field : klass.getDeclaredFields()) {
                GeneratedFromVdl annotation = field.getAnnotation(GeneratedFromVdl.class);
                if (annotation != null) {
                    hasFieldAnnotations = true;
                    break;
                }
            }
            int fieldIndex = 0;
            for (Field field : klass.getDeclaredFields()) {
                if (Modifier.isStatic(field.getModifiers())) {  // skip static fields
                    continue;
                }
                if (Character.isUpperCase(field.getName().charAt(0))) {
                    throw new IllegalArgumentException("Java field names must be lower-cased");
                }
                GeneratedFromVdl annotation = field.getAnnotation(GeneratedFromVdl.class);
                if (annotation != null) {
                    fields.put(annotation.index(), new PendingVdlField(annotation.name(),
                            lookupOrBuildPending(field.getGenericType())));
                } else if (!hasFieldAnnotations) {
                    fields.put(++fieldIndex, new PendingVdlField(firstCharToUpper(field.getName()),
                            lookupOrBuildPending(field.getGenericType())));
                }
            }
            for (Map.Entry<Integer, PendingVdlField> entry : fields.entrySet()) {
                pending.addField(entry.getValue().name, entry.getValue().type);
            }
        }

        private void populateUnion(PendingType pending, Class<?> klass) {
            pending.setKind(Kind.UNION);
            TreeMap<Integer, PendingVdlField> fields = new TreeMap<Integer, PendingVdlField>();
            for (Class<?> unionClass : klass.getDeclaredClasses()) {
                GeneratedFromVdl annotation = unionClass.getAnnotation(GeneratedFromVdl.class);
                if (annotation == null) {
                    continue;
                }
                Type type;
                try {
                    type = unionClass.getDeclaredField("elem").getGenericType();
                } catch (Exception e) {
                    throw new IllegalArgumentException(
                            "Unable to create VDL Type for type " + klass, e);
                }
                String name = annotation.name().substring(annotation.name().lastIndexOf('$') + 1);
                fields.put(annotation.index(),
                        new PendingVdlField(name, lookupOrBuildPending(type)));
            }
            for (Map.Entry<Integer, PendingVdlField> entry : fields.entrySet()) {
                pending.addField(entry.getValue().name, entry.getValue().type);
            }
        }

        private void populateArray(PendingType pending, Class<?> klass) {
            pending.setKind(Kind.ARRAY);
            Type elementType = ((ParameterizedType) klass.getGenericSuperclass())
                    .getActualTypeArguments()[0];
            pending.setElem(lookupOrBuildPending(elementType));
            try {
                ArrayLength length = klass.getAnnotation(ArrayLength.class);
                pending.setLength(length.value());
            } catch (Exception e) {
                throw new IllegalArgumentException(
                        "Unable to create VDL Type for type " + klass, e);
            }
        }

        private static String firstCharToUpper(String str) {
            return Character.toUpperCase(str.charAt(0)) + str.substring(1);
        }

        private static final class PendingVdlField {
            final String name;
            final PendingType type;
            public PendingVdlField(String name, PendingType type) {
                this.name = name;
                this.type = type;
            }
        }
    }

    /**
     * A helper class used to create {@code Type} instances for VDL types.
     */
    private static class ParameterizedTypeImpl implements ParameterizedType {
        private final Type rawType;
        private final Type[] arguments;

        public ParameterizedTypeImpl(Type rawType, Type ... arguments) {
            this.rawType = rawType;
            this.arguments = arguments;
        }

        @Override
        public Type[] getActualTypeArguments() {
            return arguments;
        }

        @Override
        public Type getRawType() {
            return rawType;
        }

        @Override
        public Type getOwnerType() {
            return null;
        }
    }
}

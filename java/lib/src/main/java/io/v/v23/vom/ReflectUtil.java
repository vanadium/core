// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vom;

import io.v.v23.vdl.VdlArray;
import io.v.v23.vdl.VdlEnum;
import io.v.v23.vdl.VdlList;
import io.v.v23.vdl.VdlMap;
import io.v.v23.vdl.VdlSet;
import io.v.v23.vdl.VdlStruct;
import io.v.v23.vdl.VdlType;
import io.v.v23.vdl.VdlValue;

import java.lang.reflect.Array;
import java.lang.reflect.Constructor;
import java.lang.reflect.GenericArrayType;
import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.ParameterizedType;
import java.lang.reflect.Type;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * ReflectUtil provides helpers to get object properties and create class instances from reflection.
 */
final class ReflectUtil {
    /**
     * Creates an instance of java primitives, one of boolean, byte, short, int, long, float,
     * double, String. Handles java types and VDL types.
     *
     * @param target the target containing java class and VDL type information
     * @param value the value of primitive to be created
     * @return an instance of VDL primitive containing the provided value if the target class
     *         is inherited from {@code VdlValue}; returns provided value otherwise
     * @throws ConversionException if the instance of the target class can't be created
     */
    static Object createPrimitive(ConversionTarget target, Object value, Class<?> valueType)
            throws ConversionException {
        Class<?> targetClass = target.getTargetClass();
        try {
            if (targetClass.getSuperclass() == VdlValue.class) {
                return targetClass.getConstructor(VdlType.class, valueType)
                        .newInstance(target.getVdlType(), value);
            } else if (VdlValue.class.isAssignableFrom(targetClass)) {
                return targetClass.getConstructor(valueType).newInstance(value);
            } else {
                return value;
            }
        } catch (Exception e) {
            throw new ConversionException(value, targetClass, e);
        }
    }

    /**
     * Creates an instance of VDL enum. The target class should be inherited from {@code VdlEnum}.
     */
    static VdlEnum createEnum(ConversionTarget target, String label) throws ConversionException {
        Class<?> targetClass = target.getTargetClass();
        if (targetClass == VdlEnum.class) {
            return new VdlEnum(target.getVdlType(), label);
        }
        try {
            return (VdlEnum) targetClass.getMethod("valueOf", String.class)
                    .invoke(null, label);
        } catch (Exception e) {
            throw new ConversionException(label, targetClass, e);
        }
    }

    private static Object createNamedGeneric(Class<?> targetClass, Object impl)
            throws ConversionException {
        try {
            if (VdlArray.class.isAssignableFrom(targetClass)) {
                return construct(targetClass, impl.getClass(), impl);
            } else if (VdlList.class.isAssignableFrom(targetClass)) {
                return construct(targetClass, List.class, impl);
            } else if (VdlMap.class.isAssignableFrom(targetClass)) {
                return construct(targetClass, Map.class, impl);
            } else if (VdlSet.class.isAssignableFrom(targetClass)) {
                return construct(targetClass, Set.class, impl);
            }
        } catch (Exception e) {
            throw new ConversionException(impl, targetClass, e);
        }
        throw new ConversionException(impl, targetClass);
     }

    private static Object construct(
            Class<?> targetClass, Class<?> ctorParamType, Object... ctorArgs)
            throws NoSuchMethodException, IllegalAccessException, InvocationTargetException,
                   InstantiationException {
        Constructor<?> ctor = targetClass.getConstructor(ctorParamType);
        ctor.setAccessible(true);
        return ctor.newInstance(ctorArgs);
    }

    /**
     * Creates an instance of generic type, one of array, list, map, set.
     *
     * @param target the target containing java class and VDL type information
     * @param impl the implementation of generic type to be created
     * @return an instance of VDL generic containing wrapped around the provided implementation
     *         object if the target class is inherited from {@code VdlValue}; returns provided
     *         implementation object otherwise
     * @throws ConversionException if the instance of the target class can't be created
     */
    @SuppressWarnings("unchecked")
    static Object createGeneric(ConversionTarget target, Object impl) throws ConversionException {
        Class<?> targetClass = target.getTargetClass();
        if (targetClass == VdlArray.class) {
            return new VdlArray<Object>(target.getVdlType(), (Object[]) impl);
        } else if (targetClass == VdlList.class) {
            return new VdlList<Object>(target.getVdlType(), (List<Object>) impl);
        } else if (targetClass == VdlSet.class) {
            return new VdlSet<Object>(target.getVdlType(), (Set<Object>) impl);
        } else if (targetClass == VdlMap.class) {
            return new VdlMap<Object, Object>(target.getVdlType(), (Map<Object, Object>) impl);
        } else if (VdlValue.class.isAssignableFrom(targetClass)) {
            return createNamedGeneric(targetClass, impl);
        } else {
            return impl;
        }
    }

    /**
     * Creates an instance of VDL struct. The target class should be inherited from
     * {@code AbstractVdlStruct}.
     */
    static Object createStruct(ConversionTarget target) throws ConversionException {
        Class<?> targetClass = target.getTargetClass();
        if (targetClass == VdlStruct.class) {
            return new VdlStruct(target.getVdlType());
        }
        try {
            Constructor<?> ctor = targetClass.getConstructor();
            ctor.setAccessible(true);
            return ctor.newInstance();
        } catch (Exception e) {
            throw new ConversionException(target.getVdlType(), targetClass, e);
        }
    }

    /**
     * Returns a {@code Class} object that is represented by provided {@code Type} object.
     */
    static Class<?> getRawClass(Type type) {
        if (type instanceof Class) {
            return (Class<?>) type;
        } else if (type instanceof ParameterizedType) {
            return getRawClass(((ParameterizedType) type).getRawType());
        } else if (type instanceof GenericArrayType) {
            Class<?> component = getRawClass(((GenericArrayType) type).getGenericComponentType());
            return Array.newInstance(component, 0).getClass();
        } else {
            return null;
        }
    }

    /**
     * Returns type of element at provided index for generic or array type.
     *
     * @param type the generic type
     * @throws ConversionException if the type has no element at index
     * @returns an array of element types; the returned array is empty if the provided type
     *          is not generic or array
     */
    static Type getElementType(Type type, int index) throws ConversionException {
        Type[] types = new Type[0];
        if (type instanceof Class) {
            Class<?> klass = (Class<?>) type;
            if (klass.isArray()) {
                types = new Type[]{klass.getComponentType()};
            } else {
                return getElementType(klass.getGenericSuperclass(), index);
            }
        } else if (type instanceof ParameterizedType) {
            types = ((ParameterizedType) type).getActualTypeArguments();
        } else if (type instanceof GenericArrayType) {
            types = new Type[]{((GenericArrayType) type).getGenericComponentType()};
        }
        if (index < 0 || index >= types.length) {
            throw new ConversionException("Type " + type + " has no element at index " + index);
        }
        return types[index];
    }

    /**
     * Sets the value of the indexed element of the specified array object to the specified value.
     *
     * @param array the array
     * @param index the index into the array
     * @param value new value of the indexed element
     * @param elementClass the class of elements in the array
     */
    static void setArrayValue(Object array, int index, Object value, Class<?> elementClass) {
        if (elementClass == Boolean.TYPE) {
            Array.setBoolean(array, index, (Boolean) value);
        } else if (elementClass == Byte.TYPE) {
            Array.setByte(array, index, (Byte) value);
        } else if (elementClass == Double.TYPE) {
            Array.setDouble(array, index, (Double) value);
        } else if (elementClass == Float.TYPE) {
            Array.setFloat(array, index, (Float) value);
        } else if (elementClass == Integer.TYPE) {
            Array.setInt(array, index, (Integer) value);
        } else if (elementClass == Long.TYPE) {
            Array.setLong(array, index, (Long) value);
        } else if (elementClass == Short.TYPE) {
            Array.setShort(array, index, (Short) value);
        } else {
            Array.set(array, index, value);
        }
    }
}

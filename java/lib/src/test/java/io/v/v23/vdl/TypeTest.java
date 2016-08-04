// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

import com.google.common.collect.ImmutableMap;
import com.google.common.reflect.TypeToken;

import junit.framework.TestCase;

import io.v.v23.vdl.VdlType.Builder;
import io.v.v23.vdl.VdlType.PendingType;

import java.lang.reflect.ParameterizedType;
import java.lang.reflect.Type;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Tests methods on VDL Types.
 */
public class TypeTest extends TestCase {
    @GeneratedFromVdl(name = "MyBool")
    private static final class MyBool extends VdlBool {
        private static final long serialVersionUID = 1L;
    }
    @GeneratedFromVdl(name = "MyByte")
    private static final class MyByte extends VdlByte {
        private static final long serialVersionUID = 1L;
    }
    @GeneratedFromVdl(name = "MyUint16")
    private static final class MyUint16 extends VdlUint16 {
        private static final long serialVersionUID = 1L;
    }
    @GeneratedFromVdl(name = "MyUint32")
    private static final class MyUint32 extends VdlUint32 {
        private static final long serialVersionUID = 1L;
    }
    @GeneratedFromVdl(name = "MyUint64")
    private static final class MyUint64 extends VdlUint64 {
        private static final long serialVersionUID = 1L;
    }
    @GeneratedFromVdl(name = "MyInt16")
    private static final class MyInt16 extends VdlInt16 {
        private static final long serialVersionUID = 1L;
    }
    @GeneratedFromVdl(name = "MyInt32")
    private static final class MyInt32 extends VdlInt32 {
        private static final long serialVersionUID = 1L;
    }
    @GeneratedFromVdl(name = "MyInt64")
    private static final class MyInt64 extends VdlInt64 {
        private static final long serialVersionUID = 1L;
    }
    @GeneratedFromVdl(name = "MyFloat32")
    private static final class MyFloat32 extends VdlFloat32 {
        private static final long serialVersionUID = 1L;
    }
    @GeneratedFromVdl(name = "MyFloat64")
    private static final class MyFloat64 extends VdlFloat64 {
        private static final long serialVersionUID = 1L;
    }
    @GeneratedFromVdl(name = "MyString")
    private static final class MyString extends VdlString {
        private static final long serialVersionUID = 1L;
    }

    @SuppressWarnings("unused")
    @GeneratedFromVdl(name = "MyUnion")
    private static class MyUnion extends VdlUnion {
        private static final long serialVersionUID = 1L;
        @GeneratedFromVdl(name = "A", index = 0)
        public static class A extends MyUnion {
            private static final long serialVersionUID = 1L;
            private MyInt16 elem;
        }
        @GeneratedFromVdl(name = "B", index = 1)
        public static class B extends MyUnion {
            private static final long serialVersionUID = 1L;
            private VdlInt32 elem;
        }
        @GeneratedFromVdl(name = "C", index = 2)
        public static class C extends MyUnion {
            private static final long serialVersionUID = 1L;
            private Long elem;
        }

        private MyUnion() {
            super(Types.getVdlTypeFromReflect(MyUnion.class), 0, null);
        }
    }

    @GeneratedFromVdl(name = "MyEnum")
    private static final class MyEnum extends VdlEnum {
        private static final long serialVersionUID = 1L;
        @GeneratedFromVdl(name = "LABEL1", index = 0)
        public static final MyEnum LABEL1 = new MyEnum("LABEL1");
        @GeneratedFromVdl(name = "LABEL2", index = 1)
        public static final MyEnum LABEL2 = new MyEnum("LABEL2");
        @GeneratedFromVdl(name = "LABEL3", index = 2)
        public static final MyEnum LABEL3 = new MyEnum("LABEL3");

        private MyEnum(String name) {
            super(Types.getVdlTypeFromReflect(MyEnum.class), name);
        }
    }

    @GeneratedFromVdl(name = "MyArray12")
    @ArrayLength(12)
    private static final class MyArray12 extends VdlArray<Set<MyUnion>> {
        private static final long serialVersionUID = 1L;

        public MyArray12(Set<MyUnion>[] value) {
            super(Types.getVdlTypeFromReflect(MyArray12.class), value);
        }
    }

    @GeneratedFromVdl(name = "MyList")
    private static final class MyList extends VdlList<List<MyArray12>> {
        private static final long serialVersionUID = 1L;
        public MyList(List<List<MyArray12>> impl) {
            super(Types.getVdlTypeFromReflect(MyList.class), impl);
        }
    }

    @GeneratedFromVdl(name = "MySet")
    private static final class MySet extends VdlSet<Set<MyList>> {
        private static final long serialVersionUID = 1L;
        public MySet(Set<Set<MyList>> impl) {
            super(Types.getVdlTypeFromReflect(MySet.class), impl);
        }
    }

    @GeneratedFromVdl(name = "MyMap")
    private static final class MyMap extends VdlMap<MyEnum, Map<MySet, MySet>> {
        private static final long serialVersionUID = 1L;
        public MyMap(Map<MyEnum, Map<MySet, MySet>> impl) {
            super(Types.getVdlTypeFromReflect(MyMap.class), impl);
        }
    }

    @GeneratedFromVdl(name = "MyStruct")
    private static final class MyStruct extends AbstractVdlStruct {
        private static final long serialVersionUID = 1L;
        @GeneratedFromVdl(name = "ByteArray", index = 0)
        private byte[][] byteArray;
        @GeneratedFromVdl(name = "Set", index = 1)
        private Set<MyMap> set;
        @GeneratedFromVdl(name = "Cycle", index = 2)
        private List<MyStruct> cycle;

        public MyStruct() {
            super(Types.getVdlTypeFromReflect(MyStruct.class));
        }
    }

    private static final Map<VdlType, Type> myTypes;

    static {
        VdlType myBool = Types.named("MyBool", Types.BOOL);
        VdlType myByte = Types.named("MyByte", Types.BYTE);
        VdlType myUint16 = Types.named("MyUint16", Types.UINT16);
        VdlType myUint32 = Types.named("MyUint32", Types.UINT32);
        VdlType myUint64 = Types.named("MyUint64", Types.UINT64);
        VdlType myInt16 = Types.named("MyInt16", Types.INT16);
        VdlType myInt32 = Types.named("MyInt32", Types.INT32);
        VdlType myInt64 = Types.named("MyInt64", Types.INT64);
        VdlType myFloat32 = Types.named("MyFloat32", Types.FLOAT32);
        VdlType myFloat64 = Types.named("MyFloat64", Types.FLOAT64);
        VdlType myString = Types.named("MyString", Types.STRING);

        VdlType myUnion = Types.named("MyUnion", Types.unionOf(new VdlField("A", myInt16),
                new VdlField("B", Types.INT32), new VdlField("C", Types.INT64)));
        VdlType myEnum = Types.named("MyEnum", Types.enumOf("LABEL1", "LABEL2", "LABEL3"));
        VdlType myArray12 = Types.named("MyArray12", Types.arrayOf(12, Types.setOf(myUnion)));
        VdlType myList = Types.named("MyList", Types.listOf(Types.listOf(myArray12)));
        VdlType mySet = Types.named("MySet", Types.setOf(Types.setOf(myList)));
        VdlType myMap = Types.named("MyMap", Types.mapOf(myEnum, Types.mapOf(mySet, mySet)));

        Builder builder = new Builder();
        PendingType pendingStruct = builder.newPending(Kind.STRUCT).setName("MyStruct");
        pendingStruct.addField("ByteArray", Types.listOf(Types.listOf(Types.BYTE)))
                .addField("Set", Types.setOf(myMap))
                .addField("Cycle", builder.listOf(pendingStruct));
        builder.build();
        VdlType myStruct = pendingStruct.built();
        VdlType myOptional = Types.optionalOf(myStruct);

        myTypes = new ImmutableMap.Builder<VdlType, Type>()
                .put(myBool, MyBool.class)
                .put(myByte, MyByte.class)
                .put(myUint16, MyUint16.class)
                .put(myUint32, MyUint32.class)
                .put(myUint64, MyUint64.class)
                .put(myInt16, MyInt16.class)
                .put(myInt32, MyInt32.class)
                .put(myInt64, MyInt64.class)
                .put(myFloat32, MyFloat32.class)
                .put(myFloat64, MyFloat64.class)
                .put(myString, MyString.class)
                .put(myUnion, MyUnion.class)
                .put(myEnum, MyEnum.class)
                .put(myArray12, MyArray12.class)
                .put(myList, MyList.class)
                .put(mySet, MySet.class)
                .put(myMap, MyMap.class)
                .put(myStruct, MyStruct.class)
                .put(myOptional, new TypeToken<VdlOptional<MyStruct>>(){}.getType())
                .build();
    }

    public void testGetVdlTypeFromReflection() {
        for (Map.Entry<VdlType, Type> entry : myTypes.entrySet()) {
            assertEquals("VDL type for reflect type: " + entry.getValue(),
                    entry.getKey(), Types.getVdlTypeFromReflect(entry.getValue()));
        }
    }

    public void testTypeString() {
        String myInt16 = "MyInt16 int16";
        String myUnion = String.format("MyUnion union{A %s;B int32;C int64}", myInt16);
        String myEnum = "MyEnum enum{LABEL1;LABEL2;LABEL3}";
        String myArray12 = String.format("MyArray12 [12]set[%s]", myUnion);
        String myList = String.format("MyList [][]%s", myArray12);
        String mySet = String.format("MySet set[set[%s]]", myList);
        String myMap = String.format("MyMap map[%s]map[%s]MySet", myEnum, mySet);
        String myStruct = String.format("MyStruct struct{ByteArray [][]byte;Set set[%s];"
                + "Cycle []MyStruct}", myMap);
        String myOptional = "?" + myStruct;

        assertEquals(myUnion, Types.getVdlTypeFromReflect(MyUnion.class).toString());
        assertEquals(myEnum, Types.getVdlTypeFromReflect(MyEnum.class).toString());
        assertEquals(myArray12, Types.getVdlTypeFromReflect(MyArray12.class).toString());
        assertEquals(myList, Types.getVdlTypeFromReflect(MyList.class).toString());
        assertEquals(mySet, Types.getVdlTypeFromReflect(MySet.class).toString());
        assertEquals(myMap, Types.getVdlTypeFromReflect(MyMap.class).toString());
        assertEquals(myStruct, Types.getVdlTypeFromReflect(MyStruct.class).toString());
        assertEquals(myOptional, Types.getVdlTypeFromReflect(
                new TypeToken<VdlOptional<MyStruct>>(){}.getType()).toString());
    }

    public void testEquals() {
        VdlType.Builder builder = new Builder();
        VdlType primitive = Types.UINT32;
        VdlType list = Types.listOf(primitive);

        PendingType recursiveSet = builder.newPending(Kind.SET).setName("recursiveSet");
        recursiveSet.setKey(recursiveSet);

        PendingType recursiveList = builder.newPending(Kind.LIST);
        PendingType recursiveStruct = builder.newPending(Kind.STRUCT).setName("recursiveStruct");
        recursiveList.setElem(recursiveStruct);
        recursiveStruct.addField("rec", recursiveSet);
        recursiveStruct.addField("rec2", recursiveList);
        builder.build();

        VdlType[] types = new VdlType[] {
                primitive, list, recursiveSet.built(),
                recursiveStruct.built(), recursiveList.built()
        };
        for (VdlType type : types) {
            for (VdlType other : types) {
                if (type == other) {
                    assertEquals(type, other);
                } else {
                    assertFalse(type.equals(other));
                }
            }
        }
    }

    private void verifyReflectType(VdlType vdlType, Class<?> ... classes) {
        Type reflectType = Types.getReflectTypeForVdl(vdlType);
        for (int i = 0; i < classes.length; i++) {
            if (i < classes.length - 1) {
                assertEquals(classes[i], ((ParameterizedType) reflectType).getRawType());
                reflectType = ((ParameterizedType) reflectType).getActualTypeArguments()[0];
            } else {
                assertEquals(classes[i], reflectType);
            }
        }
    }

    public void testGetReflectTypeForVdl() {
        verifyReflectType(Types.listOf(Types.STRING), VdlList.class, String.class);
        verifyReflectType(Types.setOf(Types.INT32), VdlSet.class, Integer.class);
        try {
            Types.getReflectTypeForVdl(Types.arrayOf(2, Types.INT16));
            fail("Reflect type should not be constructed for VDL type [2]int16");
        } catch (IllegalArgumentException expected) {
        }
        try {
            Types.getReflectTypeForVdl(Types.structOf(new VdlField("a", Types.BOOL)));
            fail("Reflect type should not be constructed for VDL type struct{a bool}");
        } catch (IllegalArgumentException expected) {
        }
    }

    public void testTypesUnique() {
        VdlType.Builder builder = new VdlType.Builder();
        PendingType p1 = builder.newPending(Kind.STRUCT);
        builder.build();
        VdlType t1 = p1.built();
        builder = new VdlType.Builder();
        PendingType p2 = builder.newPending(Kind.STRUCT);
        PendingType p3 = builder.newPending(Kind.STRUCT);
        p2.addField("SameAsT1", p3);
        builder.build();
        VdlType t2 = p2.built();
        assertEquals(t1, t2.getFields().get(0).getType());
    }
}

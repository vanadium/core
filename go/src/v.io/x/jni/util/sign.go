// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

import (
	"bytes"
	"strings"
)

type Sign string

const (
	// VoidSign denotes a signature of a Java void type.
	VoidSign Sign = "V"
	// ByteSign denotes a signature of a Java byte type.
	ByteSign Sign = "B"
	// BoolSign denotes a signature of a Java boolean type.
	BoolSign Sign = "Z"
	// CharSign denotes a signature of a Java char type.
	CharSign Sign = "C"
	// ShortSign denotes a signature of a Java short type.
	ShortSign Sign = "S"
	// IntSign denotes a signature of a Java int type.
	IntSign Sign = "I"
	// LongSign denotes a signature of a Java long type.
	LongSign Sign = "J"
	// FloatSign denotes a signature of a Java float type.
	FloatSign Sign = "F"
	// DoubleSign denotes a signature of a Java double type.
	DoubleSign Sign = "D"
)

var (
	// StringSign denotes a signature of a Java String type.
	StringSign = ClassSign("java.lang.String")
	// ObjectSign denotes a signature of a Java Object type.
	ObjectSign = ClassSign("java.lang.Object")
	// TypeSign denotes a signature of a Java Type type.
	TypeSign = ClassSign("java.lang.reflect.Type")
	// ListSign denotes a signature of a Java List type.
	ListSign = ClassSign("java.util.List")
	// CollectionSign denotes a signature of a Java Collection type.
	CollectionSign = ClassSign("java.util.Collection")
	// SetSign denotes a signature of a Java Set type.
	SetSign = ClassSign("java.util.Set")
	// MapSign denotes a signature of a Java Map type.
	MapSign = ClassSign("java.util.Map")
	// MultimapSign denotes a signature of a Guava Multimap type.
	MultimapSign = ClassSign("com.google.common.collect.Multimap")
	// IteratorSign denotes a signature of a Java Iterator type.
	IteratorSign = ClassSign("java.util.Iterator")
	// ByteArraySign denotes a signature of a Java byte array type.
	ByteArraySign = ArraySign(ByteSign)
	// DateTimeSign denotes a signature of a Java DateTime type.
	DateTimeSign = ClassSign("org.joda.time.DateTime")
	// DurationSign denotes a signature of a Java Duration type.
	DurationSign = ClassSign("org.joda.time.Duration")
	// VExceptionSign denotes a signature of a Java VException type.
	VExceptionSign = ClassSign("io.v.v23.verror.VException")
	// VDLValueSign denotes a signature of a Java VdlValue type.
	VdlValueSign = ClassSign("io.v.v23.vdl.VdlValue")
)

// ArraySign returns the array signature, given the underlying array type.
func ArraySign(sign Sign) Sign {
	return "[" + sign
}

// ClassSign returns the signature of the specified Java class.
// The class should be specified in java "java.lang.String" style.
func ClassSign(className string) Sign {
	return Sign("L" + strings.Replace(className, ".", "/", -1) + ";")
}

// FuncSign returns the signature of the specified java function.
func FuncSign(argSigns []Sign, retSign Sign) Sign {
	var buf bytes.Buffer
	buf.WriteRune('(')
	for _, sign := range argSigns {
		buf.WriteString(string(sign))
	}
	buf.WriteRune(')')
	buf.WriteString(string(retSign))
	return Sign(buf.String())
}

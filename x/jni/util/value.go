// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package util

import (
	"fmt"
	"reflect"
	"time"
)

// #include "jni_wrapper.h"
//
// jvalue jBoolValue(jboolean val) {
//   jvalue ret = { .z = val };
//   return ret;
// }
// jvalue jByteValue(jbyte val) {
//   jvalue ret = { .b = val };
//   return ret;
// }
// jvalue jCharValue(jchar val) {
//   jvalue ret = { .c = val };
//   return ret;
// }
// jvalue jShortValue(jshort val) {
//   jvalue ret = { .s = val };
//   return ret;
// }
// jvalue jIntValue(jint val) {
//   jvalue ret = { .i = val };
//   return ret;
// }
// jvalue jLongValue(jlong val) {
//   jvalue ret = { .j = val };
//   return ret;
// }
// jvalue jFloatValue(jfloat val) {
//   jvalue ret = { .f = val };
//   return ret;
// }
// jvalue jDoubleValue(jdouble val) {
//   jvalue ret = { .d = val };
//   return ret;
// }
// jvalue jObjectValue(jobject val) {
//   jvalue ret = { .l = val };
//   return ret;
// }
import "C"

var errJValue = C.jObjectValue(0)

// jValue converts a Go value into a Java value with the given sign.
func jValue(env Env, v interface{}, sign Sign) (C.jvalue, error) {
	switch sign {
	case BoolSign:
		return jBoolValue(v)
	case ByteSign:
		return jByteValue(v)
	case CharSign:
		return jCharValue(v)
	case ShortSign:
		return jShortValue(v)
	case IntSign:
		return jIntValue(v)
	case LongSign:
		return jLongValue(v)
	case StringSign:
		return jStringValue(env, v)
	case DateTimeSign:
		return jDateTimeValue(env, v)
	case DurationSign:
		return jDurationValue(env, v)
	case VExceptionSign:
		return jVExceptionValue(env, v)
	case ArraySign(ByteSign):
		return jByteArrayValue(env, v)
	case ArraySign(StringSign):
		return jStringArrayValue(env, v)
	case ArraySign(ArraySign(ByteSign)):
		return jByteArrayArrayValue(env, v)
	default:
		return jObjectValue(v)
	}
}

func jBoolValue(v interface{}) (C.jvalue, error) {
	val, ok := v.(bool)
	if !ok {
		return errJValue, fmt.Errorf("%#v isn't bool", v)
	}
	jBool := C.jboolean(C.JNI_FALSE)
	if val {
		jBool = C.jboolean(C.JNI_TRUE)
	}
	return C.jBoolValue(jBool), nil
}

func jByteValue(v interface{}) (C.jvalue, error) {
	val, ok := intValue(v)
	if !ok {
		return errJValue, fmt.Errorf("%#v isn't byte", v)
	}
	return C.jByteValue(C.jbyte(val)), nil
}

func jCharValue(v interface{}) (C.jvalue, error) {
	val, ok := intValue(v)
	if !ok {
		return errJValue, fmt.Errorf("%#v isn't char", v)
	}
	return C.jCharValue(C.jchar(val)), nil
}

func jShortValue(v interface{}) (C.jvalue, error) {
	val, ok := intValue(v)
	if !ok {
		return errJValue, fmt.Errorf("%#v isn't short", v)
	}
	return C.jShortValue(C.jshort(val)), nil
}

func jIntValue(v interface{}) (C.jvalue, error) {
	val, ok := intValue(v)
	if !ok {
		return errJValue, fmt.Errorf("%#v isn't int", v)
	}
	return C.jIntValue(C.jint(val)), nil
}

func jLongValue(v interface{}) (C.jvalue, error) {
	val, ok := intValue(v)
	if !ok {
		return errJValue, fmt.Errorf("%#v isn't long", v)
	}
	return C.jLongValue(C.jlong(val)), nil
}

func jStringValue(env Env, v interface{}) (C.jvalue, error) {
	str, ok := v.(string)
	if !ok {
		return errJValue, fmt.Errorf("%#v isn't string", v)
	}
	return jObjectValue(JString(env, str))
}

func jDateTimeValue(env Env, v interface{}) (C.jvalue, error) {
	t, ok := v.(time.Time)
	if !ok {
		return errJValue, fmt.Errorf("%#v isn't time.Time", v)
	}
	jTime, err := JTime(env, t)
	if err != nil {
		return errJValue, err
	}
	return jObjectValue(jTime)
}

func jDurationValue(env Env, v interface{}) (C.jvalue, error) {
	d, ok := v.(time.Duration)
	if !ok {
		return errJValue, fmt.Errorf("%#v isn't time.Duration", v)
	}
	jDuration, err := JDuration(env, d)
	if err != nil {
		return errJValue, err
	}
	return jObjectValue(jDuration)
}

func jVExceptionValue(env Env, v interface{}) (C.jvalue, error) {
	if v == nil {
		return C.jObjectValue(0), nil
	}
	native, ok := v.(error)
	if !ok {
		return errJValue, fmt.Errorf("%#v isn't error", v)
	}
	jVException, err := JVException(env, native)
	if err != nil {
		return errJValue, err
	}
	return jObjectValue(jVException)
}

func jByteArrayValue(env Env, v interface{}) (C.jvalue, error) {
	arr, ok := v.([]byte)
	if !ok {
		return errJValue, fmt.Errorf("%#v isn't []byte", v)
	}
	jArr, err := JByteArray(env, arr)
	if err != nil {
		return errJValue, err
	}
	return jObjectValue(jArr)
}

func jStringArrayValue(env Env, v interface{}) (C.jvalue, error) {
	arr, ok := v.([]string)
	if !ok {
		return errJValue, fmt.Errorf("%#v isn't []string", v)
	}
	jArr, err := JStringArray(env, arr)
	if err != nil {
		return errJValue, err
	}
	return jObjectValue(jArr)
}

func jByteArrayArrayValue(env Env, v interface{}) (C.jvalue, error) {
	arr, ok := v.([][]byte)
	if !ok {
		return errJValue, fmt.Errorf("%#v isn't [][]byte", v)
	}
	jArr, err := JByteArrayArray(env, arr)
	if err != nil {
		return errJValue, err
	}
	return jObjectValue(jArr)
}

func jObjectValue(v interface{}) (C.jvalue, error) {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() { // nil value
		return C.jObjectValue(0), nil
	}
	switch val := v.(type) {
	case Object:
		return C.jObjectValue(val.value()), nil
	case Class:
		return C.jObjectValue(C.jobject(val.value())), nil
	default:
		return errJValue, fmt.Errorf("%#v isn't Object or Class", v)
	}
}

func intValue(v interface{}) (int64, bool) {
	switch val := v.(type) {
	case int64:
		return val, true
	case int:
		return int64(val), true
	case int32:
		return int64(val), true
	case int16:
		return int64(val), true
	case int8:
		return int64(val), true
	case uint64:
		return int64(val), true
	case uint:
		return int64(val), true
	case uint32:
		return int64(val), true
	case uint16:
		return int64(val), true
	case uint8:
		return int64(val), true
	default:
		return 0, false
	}
}

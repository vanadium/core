// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

#include "jni_wrapper.h"

jboolean CallBooleanMethodA(JNIEnv *env, jobject obj, jmethodID methodID, jvalue *args) {
  return (*env)->CallBooleanMethodA(env, obj, methodID, args);
}

jobject CallObjectMethod(JNIEnv *env, jobject obj, jmethodID methodID) {
  return (*env)->CallObjectMethod(env,obj,methodID);
}

jobject CallObjectMethodA(JNIEnv *env, jobject obj, jmethodID methodID, jvalue *args) {
  return (*env)->CallObjectMethodA(env, obj, methodID, args);
}

void CallVoidMethod(JNIEnv *env, jobject obj, jmethodID methodID) {
  (*env)->CallVoidMethod(env, obj, methodID);
}

void CallVoidMethodA(JNIEnv *env, jobject obj, jmethodID methodID, jvalue *args) {
  (*env)->CallVoidMethodA(env, obj, methodID, args);
}

void DeleteGlobalRef(JNIEnv *env, jobject globalRef) {
  (*env)->DeleteGlobalRef(env, globalRef);
}

void ExceptionClear(JNIEnv *env) {
  (*env)->ExceptionClear(env);
}

jthrowable ExceptionOccurred(JNIEnv* env) {
  return (*env)->ExceptionOccurred(env);
}

jclass FindClass(JNIEnv* env, const char* name) {
  return (*env)->FindClass(env, name);
}

jsize GetArrayLength(JNIEnv *env, jarray array) {
  return (*env)->GetArrayLength(env, array);
}

jint GetEnv(JavaVM* jvm, JNIEnv** env, jint version) {
  return (*jvm)->GetEnv(jvm, (void**)env, version);
}

jint GetIntField(JNIEnv *env, jobject obj, jfieldID fieldID) {
  return (*env)->GetIntField(env, obj, fieldID);
}

void GetByteArrayRegion(JNIEnv *env, jbyteArray array, jsize start, jsize len, jbyte *buf) {
  return (*env)->GetByteArrayRegion(env, array, start, len, buf);
}

jfieldID GetFieldID(JNIEnv *env, jclass cls, const char *name, const char *sig) {
  return (*env)->GetFieldID(env, cls, name, sig);
}

jmethodID GetMethodID(JNIEnv* env, jclass cls, const char* name, const char* args) {
  return (*env)->GetMethodID(env, cls, name, args);
}

jclass GetObjectClass(JNIEnv *env, jobject obj) {
  return (*env)->GetObjectClass(env, obj);
}

jobject GetObjectField(JNIEnv *env, jobject obj, jfieldID fieldID) {
  return (*env)->GetObjectField(env, obj, fieldID);
}

jfieldID GetStaticFieldID(JNIEnv *env, jclass cls, const char *name, const char *sig) {
  return (*env)->GetStaticFieldID(env, cls, name, sig);
}

jobject GetStaticObjectField(JNIEnv *env, jclass cls, jfieldID fieldID)
{
  return (*env)->GetStaticObjectField(env, cls, fieldID);
}

jsize GetStringLength(JNIEnv *env, jstring string) {
  return (*env)->GetStringLength(env, string);
}

jsize GetStringUTFLength(JNIEnv *env, jstring string) {
  return (*env)->GetStringUTFLength(env, string);
}

void GetStringUTFRegion(JNIEnv *env, jstring str, jsize start, jsize len, char *buf) {
  (*env)->GetStringUTFRegion(env, str, start, len, buf);
}

jobject NewGlobalRef(JNIEnv* env, jobject obj) {
  return (*env)->NewGlobalRef(env, obj);
}

jobject NewObjectA(JNIEnv *env, jclass cls, jmethodID methodID, jvalue *args) {
  return (*env)->NewObjectA(env, cls, methodID, args);
}

void SetBooleanField(JNIEnv *env, jobject obj, jfieldID fieldID, jboolean value) {
  return (*env)->SetBooleanField(env, obj, fieldID, value);
}

void SetByteArrayRegion(JNIEnv *env, jbyteArray array, jsize start, jsize len, jbyte *buf) {
  return (*env)->SetByteArrayRegion(env, array, start, len, buf);
}

void SetLongField(JNIEnv *env, jobject obj, jfieldID fieldID, jlong value) {
  (*env)->SetLongField(env, obj, fieldID, value);
}

void SetByteField(JNIEnv *env, jobject obj, jfieldID fieldID, jbyte value) {
  (*env)->SetByteField(env, obj, fieldID, value);
}

jbyteArray NewByteArray(JNIEnv *env, jsize length) {
  return (*env)->NewByteArray(env, length);
}

jstring NewStringUTF(JNIEnv *env, const char *bytes) {
  return (*env)->NewStringUTF(env, bytes);
}

void SetObjectField(JNIEnv *env, jobject obj, jfieldID fieldID, jobject value) {
  (*env)->SetObjectField(env, obj, fieldID, value);
}

jint Throw(JNIEnv *env, jthrowable obj) {
  return (*env)->Throw(env, obj);
}

jint ThrowNew(JNIEnv *env, jclass cls, const char *message) {
  return (*env)->ThrowNew(env, cls, message);
}

jint PushLocalFrame(JNIEnv *env, jint capacity) {
  return (*env)->PushLocalFrame(env, capacity);
}

jobject PopLocalFrame(JNIEnv *env, jobject result) {
  return (*env)->PopLocalFrame(env, result);
}

void ExceptionDescribe(JNIEnv *env) {
  (*env)->ExceptionDescribe(env);
}
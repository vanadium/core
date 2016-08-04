// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

#include "jni_wrapper.h"

jobject NewObjectA(JNIEnv *env, jclass clazz, jmethodID methodID, jvalue *args) {
  return (*env)->NewObjectA(env, clazz, methodID, args);
}

jobject CallObjectMethodA(JNIEnv *env, jobject obj, jmethodID methodID, jvalue *args) {
  return (*env)->CallObjectMethodA(env, obj, methodID, args);
}

jint CallIntMethodA(JNIEnv *env, jobject obj, jmethodID methodID, jvalue *args) {
  return (*env)->CallIntMethodA(env, obj, methodID, args);
}

jlong CallLongMethodA(JNIEnv *env, jobject obj, jmethodID methodID, jvalue *args) {
  return (*env)->CallLongMethodA(env, obj, methodID, args);
}

jboolean CallBooleanMethodA(JNIEnv *env, jobject obj, jmethodID methodID, jvalue *args) {
  return (*env)->CallBooleanMethodA(env, obj, methodID, args);
}

void CallVoidMethodA(JNIEnv *env, jobject obj, jmethodID methodID, jvalue *args) {
  (*env)->CallVoidMethodA(env, obj, methodID, args);
}

jobject CallStaticObjectMethodA(JNIEnv *env, jclass cls, jmethodID methodID, jvalue *args) {
  return (*env)->CallStaticObjectMethodA(env, cls, methodID, args);
}
jint CallStaticIntMethodA(JNIEnv *env, jclass cls, jmethodID methodID, jvalue *args) {
  return (*env)->CallStaticIntMethodA(env, cls, methodID, args);
}
jlong CallStaticLongMethodA(JNIEnv *env, jclass cls, jmethodID methodID, jvalue *args) {
  return (*env)->CallStaticLongMethodA(env, cls, methodID, args);
}
jboolean CallStaticBooleanMethodA(JNIEnv *env, jclass cls, jmethodID methodID, jvalue *args) {
  return (*env)->CallStaticBooleanMethodA(env, cls, methodID, args);
}
void CallStaticVoidMethodA(JNIEnv *env, jclass cls, jmethodID methodID, jvalue *args) {
  return (*env)->CallStaticVoidMethodA(env, cls, methodID, args);
}

jclass GetObjectClass(JNIEnv* env, jobject obj) {
  return (*env)->GetObjectClass(env, obj);
}

jclass FindClass(JNIEnv* env, const char* name) {
  return (*env)->FindClass(env, name);
}

jmethodID GetMethodID(JNIEnv* env, jclass class, const char* name, const char* args) {
  return (*env)->GetMethodID(env, class, name, args);
}

jmethodID GetStaticMethodID(JNIEnv* env, jclass class, const char* name, const char* args) {
  return (*env)->GetStaticMethodID(env, class, name, args);
}

jfieldID GetFieldID(JNIEnv* env, jclass class, const char* name, const char* sig) {
  return (*env)->GetFieldID(env, class, name, sig);
}

jfieldID GetStaticFieldID(JNIEnv* env, jclass class, const char* name, const char* sig) {
  return (*env)->GetStaticFieldID(env, class, name, sig);
}

jobject GetObjectField(JNIEnv* env, jobject obj, jfieldID fieldID) {
  return (*env)->GetObjectField(env, obj, fieldID);
}

jboolean GetBooleanField(JNIEnv* env, jobject obj, jfieldID fieldID) {
  return (*env)->GetBooleanField(env, obj, fieldID);
}

jint GetIntField(JNIEnv* env, jobject obj, jfieldID fieldID) {
  return (*env)->GetIntField(env, obj, fieldID);
}

jlong GetLongField(JNIEnv* env, jobject obj, jfieldID fieldID) {
  return (*env)->GetLongField(env, obj, fieldID);
}

jobject GetStaticObjectField(JNIEnv* env, jclass cls, jfieldID fieldID) {
  return (*env)->GetStaticObjectField(env, cls, fieldID);
}

jobjectArray NewObjectArray(JNIEnv* env, jsize len, jclass class, jobject initialElement) {
  return (*env)->NewObjectArray(env, len, class, initialElement);
}

jbyteArray NewByteArray(JNIEnv* env, jsize len) {
  return (*env)->NewByteArray(env, len);
}

jsize GetArrayLength(JNIEnv* env, jarray array) {
  return (*env)->GetArrayLength(env, array);
}

jobject GetObjectArrayElement(JNIEnv* env, jobjectArray array, jsize index) {
  return (*env)->GetObjectArrayElement(env, array, index);
}

void SetObjectArrayElement(JNIEnv* env, jobjectArray array, jsize index, jobject obj) {
  (*env)->SetObjectArrayElement(env, array, index, obj);
}

jbyte* GetByteArrayElements(JNIEnv* env, jbyteArray array, jboolean *isCopy) {
  return (*env)->GetByteArrayElements(env, array, isCopy);
}

jlong* GetLongArrayElements(JNIEnv* env, jlongArray array, jboolean *isCopy) {
  return (*env)->GetLongArrayElements(env, array, isCopy);
}

void ReleaseByteArrayElements(JNIEnv* env, jbyteArray array, jbyte* elems, jint mode) {
  (*env)->ReleaseByteArrayElements(env, array, elems, mode);
}

void ReleaseLongArrayElements(JNIEnv* env, jlongArray array, jlong* elems, jint mode) {
  (*env)->ReleaseLongArrayElements(env, array, elems, mode);
}

void SetByteArrayRegion(JNIEnv* env, jbyteArray array, jsize start, jsize len, const jbyte* data) {
  (*env)->SetByteArrayRegion(env, array, start, len, data);
}

const char* GetStringUTFChars(JNIEnv* env, jstring str, jboolean* isCopy) {
  return (*env)->GetStringUTFChars(env, str, isCopy);
}

void ReleaseStringUTFChars(JNIEnv* env, jstring str, const char* utf) {
  (*env)->ReleaseStringUTFChars(env, str, utf);
}

jstring NewStringUTF(JNIEnv* env, const char* str) {
  return (*env)->NewStringUTF(env, str);
}

jint Throw(JNIEnv* env, jthrowable obj) {
  return (*env)->Throw(env, obj);
}

jint ThrowNew(JNIEnv* env, jclass class, const char* msg) {
  return (*env)->ThrowNew(env, class, msg);
}

jobject NewLocalRef(JNIEnv* env, jobject obj) {
  return (*env)->NewLocalRef(env, obj);
}

void DeleteLocalRef(JNIEnv* env, jobject localRef) {
  (*env)->DeleteLocalRef(env, localRef);
}

jobject NewGlobalRef(JNIEnv* env, jobject obj) {
  return (*env)->NewGlobalRef(env, obj);
}

void DeleteGlobalRef(JNIEnv* env, jobject globalRef) {
  (*env)->DeleteGlobalRef(env, globalRef);
}

jobjectRefType GetObjectRefType(JNIEnv* env, jobject obj) {
  return (*env)->GetObjectRefType(env, obj);
}

jint GetJavaVM(JNIEnv* env, JavaVM** vm) {
  return (*env)->GetJavaVM(env, vm);
}

jthrowable ExceptionOccurred(JNIEnv* env) {
  return (*env)->ExceptionOccurred(env);
}

void ExceptionClear(JNIEnv* env) {
  return (*env)->ExceptionClear(env);
}

void ExceptionDescribe(JNIEnv* env) {
  return (*env)->ExceptionDescribe(env);
}

jint AttachCurrentThread(JavaVM* jvm, JNIEnv** env, void* args) {
  return (*jvm)->AttachCurrentThread(jvm, (void**) env, args);
}

jint AttachCurrentThreadAsDaemon(JavaVM* jvm, JNIEnv** env, void* args) {
  return (*jvm)->AttachCurrentThreadAsDaemon(jvm, (void**) env, args);
}

jint DetachCurrentThread(JavaVM* jvm) {
  return (*jvm)->DetachCurrentThread(jvm);
}

jint GetEnv(JavaVM* jvm, JNIEnv** env, jint version) {
  return (*jvm)->GetEnv(jvm, (void**)env, version);
}

jboolean IsInstanceOf(JNIEnv *env, jobject obj, jclass class) {
  return (*env)->IsInstanceOf(env, obj, class);
}

jint PushLocalFrame(JNIEnv *env, jint capacity) {
  return (*env)->PushLocalFrame(env, capacity);
}

jobject PopLocalFrame(JNIEnv *env, jobject result) {
  return (*env)->PopLocalFrame(env, result);
}

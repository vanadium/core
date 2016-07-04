// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android cgo

// All JNI functions are function pointers of a JNIEnv variable. Go language
// cannot call function pointers so we need use some wrapper functions to do
// that.

#include "jni.h"

jint AttachCurrentThread(JavaVM *jvm, JNIEnv **env, void *args);
jint AttachCurrentThreadAsDaemon(JavaVM* jvm, JNIEnv** env, void* args);
jboolean CallBooleanMethodA(JNIEnv *env, jobject obj, jmethodID methodID, jvalue *args);
jobject CallIntMethod(JNIEnv *env, jobject obj, jmethodID methodID);
jobject CallObjectMethod(JNIEnv *env, jobject obj, jmethodID methodID);
jobject CallObjectMethodA(JNIEnv *env, jobject obj, jmethodID methodID, jvalue *args);
void CallVoidMethod(JNIEnv *env, jobject obj, jmethodID methodID);
void CallVoidMethodA(JNIEnv *env, jobject obj, jmethodID methodID, jvalue *args);
void DeleteGlobalRef(JNIEnv *env, jobject globalRef);
void ExceptionClear(JNIEnv *env);
jthrowable ExceptionOccurred(JNIEnv* env);
jclass FindClass(JNIEnv* env, const char* name);
jsize GetArrayLength(JNIEnv *env, jarray array);
jint GetEnv(JavaVM* jvm, JNIEnv** env, jint version);
jint GetIntField(JNIEnv *env, jobject obj, jfieldID fieldID);
void GetByteArrayRegion(JNIEnv *env, jbyteArray array, jsize start, jsize len, jbyte *buf);
jfieldID GetFieldID(JNIEnv *env, jclass cls, const char *name, const char *sig);
jmethodID GetMethodID(JNIEnv* env, jclass cls, const char* name, const char* sig);
jclass GetObjectClass(JNIEnv *env, jobject obj);
jobject GetObjectField(JNIEnv *env, jobject obj, jfieldID fieldID);
jfieldID GetStaticFieldID(JNIEnv *env, jclass cls, const char *name, const char *sig);
jobject GetStaticObjectField(JNIEnv *env, jclass cls, jfieldID fieldID);
jsize GetStringLength(JNIEnv *env, jstring string);
jsize GetStringUTFLength(JNIEnv *env, jstring string);
void GetStringUTFRegion(JNIEnv *env, jstring str, jsize start, jsize len, char *buf);
jbyteArray NewByteArray(JNIEnv *env, jsize length);
jobject NewGlobalRef(JNIEnv* env, jobject obj);
jobject NewObjectA(JNIEnv *env, jclass cls, jmethodID methodID, jvalue *args);
jstring NewStringUTF(JNIEnv *env, const char *bytes);
void SetBooleanField(JNIEnv *env, jobject obj, jfieldID fieldID, jboolean value);
void SetByteField(JNIEnv *env, jobject obj, jfieldID fieldID, jbyte value);
void SetByteArrayRegion(JNIEnv *env, jbyteArray array, jsize start, jsize len, jbyte *buf);
void SetLongField(JNIEnv *env, jobject obj, jfieldID fieldID, jlong value);
void SetObjectField(JNIEnv *env, jobject obj, jfieldID fieldID, jobject value);
jint Throw(JNIEnv *env, jthrowable obj);
jint ThrowNew(JNIEnv *env, jclass cls, const char *message);

jint PushLocalFrame(JNIEnv *env, jint capacity);
jobject PopLocalFrame(JNIEnv *env, jobject result);
void ExceptionDescribe(JNIEnv *env);
// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build android

#include "jni_wrapper.h"

jint AttachCurrentThread(JavaVM *jvm, JNIEnv **env, void *args) {
  return (*jvm)->AttachCurrentThread(jvm, env, args);
}

jint AttachCurrentThreadAsDaemon(JavaVM *jvm, JNIEnv **env, void *args) {
  return (*jvm)->AttachCurrentThreadAsDaemon(jvm, env, args);
}
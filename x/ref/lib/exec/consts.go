// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package exec

// The V23_EXEC_CONFIG environment variable is used to share a base64 encoded JSON
// dictionary containing an instance of Config between a parent and child process.
const V23_EXEC_CONFIG = "V23_EXEC_CONFIG" //nolint:golint

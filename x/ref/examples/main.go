// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// An example using the 'library' factory which is configured via exported
// variables rather than by command line flags.
package main

import (
  "v.io/v23"
  "v.io/x/ref/runtime/factories/library"
)

func init() {
  library.ListenProxy = "test-proxy"
  library.AlsoLogToStderr = true
}

func main() {
  ctx, shutdown := v23.Init()
  ctx.Infof("Listen spec %v", v23.GetListenSpec(ctx))
  principal := v23.GetPrincipal(ctx)
  ctx.Infof("Principal %s", principal.PublicKey())

  defer shutdown()
}

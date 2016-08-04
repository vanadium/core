// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package runtime and its subdirectories provide implementations of the
// Vanadium runtime for different runtime environments.
//
// Each subdirectory of the runtime/factories package is a package that
// implements the v23.RuntimeFactory function.
//
// runtime/internal has common functionality use in runtime/factories.
//
// RuntimeFactories register themselves by calling v.io/v23/rt.RegisterRuntimeFactory
// in an init function.  Users choose a particular RuntimeFactory implementation
// by importing the appropriate package in their main package.  It is an error
// to import more than one RuntimeFactory, and the registration mechanism will
// panic if this is attempted.
//
// The 'roaming' RuntimeFactory adds operating system support for varied network
// configurations and in particular dhcp. It should be used by any application
// that may 'roam' or be behind a 1-1 NAT.
//
// The 'static' RuntimeFactory does not provide dhcp support, but is otherwise
// like the 'roaming' RuntimeFactory.
package runtime

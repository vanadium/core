// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
Command vdltestgen generates types and values for the vdltest package.  The
following files are generated:

   vtype_gen.vdl       - Named "V" types, regular VDL types.
   ventry_pass_gen.vdl - Entries that pass conversion from source to target.
   ventry_fail_gen.vdl - Entries that fail conversion from source to target.

   xtype_gen.vdl       - Named "X" types, no VDL{IsZero,Read,Write} methods.
   xentry_pass_gen.vdl - Entries that pass conversion from source to target.
   xentry_fail_gen.vdl - Entries that fail conversion from source to target.

Do not run this tool manually.  Instead invoke it via:
   $ jiri run go generate v.io/v23/vdl/vdltest

Usage:
   vdltestgen [flags]

The global flags are:
 -metadata=<just specify -metadata to activate>
   Displays metadata for the program and exits.
 -time=false
   Dump timing information to stderr before exiting the program.
 -vdltest=
   Filter vdltest.All to only return entries that contain the given substring.
   If the value starts with "!", only returns entries that don't contain the
   given substring.
*/
package main

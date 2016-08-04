#!/bin/bash
# Copyright 2016 The Vanadium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# Runs vdltestgen to re-generate vdltest data files.

# Don't set -f, since we need wildcard expansion.
set -eu -o pipefail

# First remove all generated files, since vdltestgen depends on the vdltest
# package.  It's annoying during development if we generate invalid code, since
# we won't be able to rebuild vdltestgen in order generate new code.  By
# removing the generated files we avoid this problem, and also speed up the
# build of vdltestgen.
rm -f *_gen.vdl *_gen.go

# Since we removed the generated files above, we need to write a dummy file that
# contains the variables necessary for the vdltest package to compile.
dummy_file="./dummy_gen.go"
cat - > ${dummy_file} <<EOF
package vdltest

// This dummy file is only used when compiling the vdltestgen tool to generate
// new test entries.  Normally these vars are defined in *_gen.vdl files.
var vAllPass, vAllFail, xAllPass, xAllFail []vdlEntry
EOF

# Re-generate the vdltest package, since we removed the vdl files above.
jiri run go install "v.io/x/ref/cmd/vdl"
jiri run "${JIRI_ROOT}/release/go/bin/vdl" generate "v.io/v23/vdl/vdltest"

# Install and run vdltestgen
jiri run go install "v.io/v23/vdl/vdltest/internal/vdltestgen"
jiri run "${JIRI_ROOT}/release/go/bin/vdltestgen"

# Re-generate the vdltest package, now with the new vdl files.
jiri run "${JIRI_ROOT}/release/go/bin/vdl" generate "v.io/v23/vdl/vdltest"

# Clean up temporary files
rm -f ${dummy_file}

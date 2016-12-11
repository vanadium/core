#!/bin/bash
# Copyright 2016 The Vanadium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# Runs vomtestgen to re-generate vomtest data files.

# Don't set -f, since we need wildcard expansion.
set -eu -o pipefail

# Re-generate the vdltest package, to make sure it's up-to-date.
jiri run go generate "v.io/v23/vdl/vdltest"

# Install and run vomtestgen
jiri run go install "v.io/v23/vom/vomtest/internal/vomtestgen"
jiri run "${JIRI_ROOT}/release/go/bin/vomtestgen"

# Re-generate the vomtest package, now with the new vdl files.
jiri run go install "v.io/x/ref/cmd/vdl"
jiri run "${JIRI_ROOT}/release/go/bin/vdl" generate "v.io/v23/vom/vomtest"

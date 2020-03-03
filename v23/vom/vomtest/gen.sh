#!/bin/bash
# Copyright 2016 The Vanadium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# Runs vomtestgen to re-generate vomtest data files.

# Don't set -f, since we need wildcard expansion.
set -eu -o pipefail

# Re-generate the vdltest package, to make sure it's up-to-date.
go generate "v.io/v23/vdl/vdltest"

# Install and run vomtestgen
go run "v.io/v23/vom/vomtest/internal/vomtestgen"

# Re-generate the vomtest package, now with the new vdl files.
go run "v.io/x/ref/cmd/vdl" generate "v.io/v23/vom/vomtest"

#!/bin/bash
# Copyright 2016 The Vanadium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# Generate the builtin_vdlroot.go source file, by running go-bindata.

set -euf -o pipefail

# Install go-bindata
jiri go install github.com/jteeuwen/go-bindata/...

PREFIX="${JIRI_ROOT}/release/go/src"
VDLROOT="${PREFIX}/v.io/v23/vdlroot"
X="${PREFIX}/v.io/x/ref/lib/vdl/build/internal/builtin_vdlroot/builtin_vdlroot"
OUT="${X}.go"
TMP="${X}.tmp.go"

# The cwd must be set here in order for paths start with "v.io/v23/vdlroot/..."
cd "${PREFIX}"

# Run go-bindata to generate the file to a tmp file.
jiri run "${JIRI_ROOT}/third_party/go/bin/go-bindata" -o "${TMP}" -pkg builtin_vdlroot -prefix "${VDLROOT}" -ignore '(\.api|\.go)' -nometadata -mode 0644 "${VDLROOT}/..."

# Format the file and add the copyright header.
gofmt -l -w "${TMP}"
cat - "${TMP}" >  "${OUT}" <<EOF
// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

EOF

# Remove the tmp file.
rm "${TMP}"

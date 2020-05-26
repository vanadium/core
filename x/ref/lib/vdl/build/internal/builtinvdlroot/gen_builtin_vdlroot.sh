#!/bin/bash
# Copyright 2016 The Vanadium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# Generate the builtin_vdlroot.go source file, by running go-bindata.

set -euf -o pipefail

go get github.com/cosnicolaou/go-bindata/v3/...@v3.0.8

PREFIX=$(dirname "$(go env GOMOD)")
VDLROOT="${PREFIX}/v23/vdlroot"
X="${PREFIX}/x/ref/lib/vdl/build/internal/builtinvdlroot/builtin_vdlroot"

OUT="${X}.go"
TMP="${X}.tmp.go"

# The cwd must be set here in order for paths start with "v.io/v23/vdlroot/..."
cd "${PREFIX}"

# Run go-bindata to generate the file to a tmp file.
go run github.com/cosnicolaou/go-bindata/v3/go-bindata -o "${TMP}" -pkg builtinvdlroot -prefix "${VDLROOT}" -ignore '(\.api|\.go)' -nometadata -mode 0644 "${VDLROOT}/..."

# Format the file and add the copyright header.
gofmt -s -l -w "${TMP}"
cat - "${TMP}" >  "${OUT}" <<EOF
// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

EOF

# Remove the tmp files.
rm "${TMP}" 

#!/bin/bash
# Copyright 2016 The Vanadium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# Generate the assets.go source file, by running go-bindata.

set -euf -o pipefail

# Install go-bindata.
go get "github.com/cosnicolaou/go-bindata/v3/...@v3.0.8"

cd "$(dirname $0)"

OUT="assets.go"
TMP=$(mktemp "XXXXXXXXXX_assets.go")

# Run go-bindata to generate the file to a tmp file.
go run "github.com/cosnicolaou/go-bindata/v3/go-bindata" \
    -o "${TMP}" -pkg browseserver -prefix "assets" \
    -nometadata -mode 0644 "assets/..."

# Format the file and add the copyright header.
go fmt "${TMP}" > /dev/null
cat - "${TMP}" >  "${OUT}" <<EOF
// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

EOF

# Remove the tmp file and dir.
rm -rf "${TMP}"

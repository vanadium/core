#!/bin/bash
# Copyright 2016 The Vanadium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.


go run $JIRI_ROOT/release/go/src/v.io/v23/vom/internal/gen/gen.go -- $JIRI_ROOT/release/go/src/v.io/v23/vom/internal/bench_test.go
go fmt $JIRI_ROOT/release/go/src/v.io/v23/vom/internal/bench_test.go
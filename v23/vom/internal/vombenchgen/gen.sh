#!/bin/bash
# Copyright 2016 The Vanadium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

go install v.io/v23/vom/internal/vombenchgen
vombenchgen -- ../bench_test.go
go fmt ../bench_test.go

// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// struct tags defined in config.vdl are not supported in bootstrapping
// mode. This file contains the functions used for normal operation.
//
//go:build !vdltoolbootstrapping
// +build !vdltoolbootstrapping

package golang

func structTagFor(data *goData, structName, fieldName string) (string, bool) {
	if data.Package == nil || data.Package.Config.Go.StructTags == nil {
		return "", false
	}
	structTags := data.Package.Config.Go.StructTags
	if tags, ok := structTags[structName]; ok {
		for _, tag := range tags {
			if tag.Field == fieldName {
				return tag.Tag, true
			}
		}
	}
	return "", false
}

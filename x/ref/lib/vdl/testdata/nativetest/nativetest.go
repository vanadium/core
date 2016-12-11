// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nativetest

import (
	"strconv"
	"time"
)

func WireStringToNative(x WireString, native *string) error {
	*native = strconv.Itoa(int(x))
	return nil
}
func WireStringFromNative(x *WireString, native string) error {
	v, err := strconv.Atoi(native)
	*x = WireString(v)
	return err
}

func WireTimeToNative(WireTime, *time.Time) error   { return nil }
func WireTimeFromNative(*WireTime, time.Time) error { return nil }

func WireSamePkgToNative(WireSamePkg, native *NativeSamePkg) error { return nil }
func WireSamePkgFromNative(*WireSamePkg, NativeSamePkg) error      { return nil }

func WireMultiImportToNative(WireMultiImport, *map[NativeSamePkg]time.Time) error   { return nil }
func WireMultiImportFromNative(*WireMultiImport, map[NativeSamePkg]time.Time) error { return nil }

type NativeSamePkg string

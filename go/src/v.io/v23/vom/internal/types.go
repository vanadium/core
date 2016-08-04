// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"v.io/v23/vom"
)

type XNumber int32
type XString string
type XByteList []byte
type XByteArray [3]byte
type XArray [3]int32
type XList []int32
type XListAny []*vom.RawBytes
type XMap map[string]bool
type XSmallStruct struct {
	A int32
	B string
	C bool
}
type XLargeStruct struct {
	F1  int32
	F2  int32
	F3  int32
	F4  int32
	F5  int32
	F6  int32
	F7  int32
	F8  int32
	F9  int32
	F10 int32
	F11 int32
	F12 int32
	F13 int32
	F14 int32
	F15 int32
	F16 int32
	F17 int32
	F18 int32
	F19 int32
	F20 int32
	F21 int32
	F22 int32
	F23 int32
	F24 int32
	F25 int32
	F26 int32
	F27 int32
	F28 int32
	F29 int32
	F30 int32
	F31 int32
	F32 int32
	F33 int32
	F34 int32
	F35 int32
	F36 int32
	F37 int32
	F38 int32
	F39 int32
	F40 int32
	F41 int32
	F42 int32
	F43 int32
	F44 int32
	F45 int32
	F46 int32
	F47 int32
	F48 int32
	F49 int32
	F50 int32
}

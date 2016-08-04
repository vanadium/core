// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"crypto/md5"
	crand "crypto/rand"
	"encoding/binary"
	"math/rand"

	"v.io/x/ref/runtime/internal/rpc/stress"
)

// newSumArg returns a randomly generated SumArg.
func newSumArg(maxPayloadSize int) (stress.SumArg, error) {
	var arg stress.SumArg
	arg.ABool = rand.Intn(2) == 0
	arg.AInt64 = rand.Int63()
	arg.AListOfBytes = make([]byte, rand.Intn(maxPayloadSize)+1)
	_, err := crand.Read(arg.AListOfBytes)
	return arg, err
}

// lenSumArg returns the length of the SumArg in bytes.
func lenSumArg(arg *stress.SumArg) int {
	// bool + uint64 + []byte
	return 1 + 4 + len(arg.AListOfBytes)
}

// doSum returns the MD5 checksum of the SumArg.
func doSum(arg *stress.SumArg) ([]byte, error) {
	h := md5.New()
	if arg.ABool {
		if err := binary.Write(h, binary.LittleEndian, arg.AInt64); err != nil {
			return nil, err
		}
	}
	if _, err := h.Write(arg.AListOfBytes); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

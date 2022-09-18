// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"bytes"
	"io"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/vom"
	"v.io/x/ref/test"
)

func setupCredentials(b *testing.B, ctx *context.T) (security.Blessings, []security.Discharge) {
	p := v23.GetPrincipal(ctx)
	blessings, _ := p.BlessingStore().Default()

	keyCav, err := security.NewPublicKeyCaveat(p.PublicKey(),
		"peoria",
		security.ThirdPartyRequirements{},
		security.UnconstrainedUse())
	if err != nil {
		b.Fatal(err)
	}
	expiryCav, err := security.NewExpiryCaveat(time.Now().Add(time.Second))
	if err != nil {
		b.Fatal(err)
	}
	cav3p, _ := security.NewPublicKeyCaveat(p.PublicKey(), "somewhere", security.ThirdPartyRequirements{}, expiryCav)
	discharge, err := p.MintDischarge(keyCav, security.UnconstrainedUse(), cav3p)
	if err != nil {
		b.Fatal(err)
	}
	return blessings, []security.Discharge{discharge}
}

func benchmarkEncodeOnly(b *testing.B) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	blessings, discharges := setupCredentials(b, ctx)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		enc := vom.NewEncoder(io.Discard)
		err := enc.Encode(BlessingsFlowMessageBlessings{Blessings{
			BKey:      3,
			Blessings: blessings,
		}})
		if err != nil {
			b.Fatal(err)
		}
		err = enc.Encode(BlessingsFlowMessageDischarges{Discharges{
			Discharges: discharges,
			DKey:       2,
			BKey:       3,
		}})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkDecodeOnly(b *testing.B) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	blessings, discharges := setupCredentials(b, ctx)
	bufs := make([]*bytes.Buffer, b.N)
	for i := 0; i < b.N; i++ {
		bufs[i] = bytes.NewBuffer(make([]byte, 0, 4096))
		enc := vom.NewEncoder(bufs[i])
		err := enc.Encode(BlessingsFlowMessageBlessings{Blessings{
			BKey:      3,
			Blessings: blessings,
		}})
		if err != nil {
			b.Fatal(err)
		}
		err = enc.Encode(BlessingsFlowMessageDischarges{Discharges{
			Discharges: discharges,
			DKey:       2,
			BKey:       3,
		}})
		if err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dec := vom.NewDecoder(bufs[i])
		var r BlessingsFlowMessage
		if err := dec.Decode(&r); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkEncodeOnlyStruct(b *testing.B) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	blessings, discharges := setupCredentials(b, ctx)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		enc := vom.NewEncoder(io.Discard)
		err := enc.Encode(Blessings{
			BKey:      3,
			Blessings: blessings,
		})
		if err != nil {
			b.Fatal(err)
		}
		err = enc.Encode(Discharges{
			Discharges: discharges,
			DKey:       2,
			BKey:       3,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkDecodeOnlyStruct(b *testing.B) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	blessings, discharges := setupCredentials(b, ctx)
	bufs := make([]*bytes.Buffer, b.N)
	for i := 0; i < b.N; i++ {
		bufs[i] = bytes.NewBuffer(make([]byte, 0, 4096))
		enc := vom.NewEncoder(bufs[i])
		err := enc.Encode(Blessings{
			BKey:      3,
			Blessings: blessings,
		})
		if err != nil {
			b.Fatal(err)
		}
		err = enc.Encode(Discharges{
			Discharges: discharges,
			DKey:       2,
			BKey:       3,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dec := vom.NewDecoder(bufs[i])
		var rb Blessings
		if err := dec.Decode(&rb); err != nil {
			b.Fatal(err)
		}
		var rd Discharges
		if err := dec.Decode(&rd); err != nil {
			b.Fatal(err)
		}
	}
}

func Benchmark_BlessingsFlow__Encode____Union(b *testing.B) {
	b.ReportAllocs()
	benchmarkEncodeOnly(b)
}

func Benchmark_BlessingsFlow__Decode____Union(b *testing.B) {
	b.ReportAllocs()
	benchmarkDecodeOnly(b)
}

func Benchmark_BlessingsFlow__Encode__Structs(b *testing.B) {
	b.ReportAllocs()
	benchmarkEncodeOnlyStruct(b)
}

func Benchmark_BlessingsFlow__Decode__Structs(b *testing.B) {
	b.ReportAllocs()
	benchmarkDecodeOnlyStruct(b)
}

func Benchmark_Map_Empty(b *testing.B) {
	type dummy struct{}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		mp := map[*dummy]struct{}{}
		for j := 0; j < 300; j++ {
			n := &dummy{}
			mp[n] = struct{}{}
		}
	}
}

func Benchmark_Map_Bool(b *testing.B) {
	type dummy struct{}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		mp := map[*dummy]bool{}
		for j := 0; j < 300; j++ {
			n := &dummy{}
			mp[n] = true
		}
	}
}

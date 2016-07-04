// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/vom"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/syncbase/server/interfaces"
	"v.io/x/ref/services/syncbase/store/watchable"
)

// TestDiffGenVectors tests diffing gen vectors.
func TestDiffPrefixGenVectors(t *testing.T) {
	svc := createService(t)
	defer destroyService(t, svc)
	s := svc.sync
	s.id = 10 //responder. Initiator is id 11.

	tests := []struct {
		respVec, initVec interfaces.GenVector
		genDiffIn        genRangeVector
		genDiffWant      genRangeVector
	}{
		{ // responder and initiator are at identical vectors.
			respVec:   interfaces.GenVector{10: 1, 11: 10, 12: 20, 13: 2},
			initVec:   interfaces.GenVector{10: 1, 11: 10, 12: 20, 13: 2},
			genDiffIn: make(genRangeVector),
		},
		{ // responder and initiator are at identical vectors.
			respVec:   interfaces.GenVector{10: 0},
			initVec:   interfaces.GenVector{10: 0},
			genDiffIn: make(genRangeVector),
		},
		{ // responder has no updates.
			respVec:   interfaces.GenVector{10: 0},
			initVec:   interfaces.GenVector{10: 5, 11: 10, 12: 20, 13: 8},
			genDiffIn: make(genRangeVector),
		},
		{ // responder and initiator have no updates.
			respVec:   interfaces.GenVector{10: 0},
			initVec:   interfaces.GenVector{11: 0},
			genDiffIn: make(genRangeVector),
		},
		{ // responder is staler than initiator.
			respVec:   interfaces.GenVector{10: 1, 11: 10, 12: 20, 13: 2},
			initVec:   interfaces.GenVector{10: 1, 11: 10, 12: 20, 13: 8, 14: 5},
			genDiffIn: make(genRangeVector),
		},
		{ // responder is more up-to-date than initiator for local updates.
			respVec:     interfaces.GenVector{10: 5, 11: 10, 12: 20, 13: 2},
			initVec:     interfaces.GenVector{10: 1, 11: 10, 12: 20, 13: 2},
			genDiffIn:   make(genRangeVector),
			genDiffWant: genRangeVector{10: &genRange{min: 2, max: 5}},
		},
		{ // responder is fresher than initiator for local updates and one device.
			respVec:   interfaces.GenVector{10: 5, 11: 10, 12: 22, 13: 2},
			initVec:   interfaces.GenVector{10: 1, 11: 10, 12: 20, 13: 2, 14: 40},
			genDiffIn: make(genRangeVector),
			genDiffWant: genRangeVector{
				10: &genRange{min: 2, max: 5},
				12: &genRange{min: 21, max: 22},
			},
		},
		{ // responder is fresher than initiator in all but one device.
			respVec:   interfaces.GenVector{10: 1, 11: 2, 12: 3, 13: 4},
			initVec:   interfaces.GenVector{10: 0, 11: 2, 12: 0},
			genDiffIn: make(genRangeVector),
			genDiffWant: genRangeVector{
				10: &genRange{min: 1, max: 1},
				12: &genRange{min: 1, max: 3},
				13: &genRange{min: 1, max: 4},
			},
		},
		{ // initiator has no updates.
			respVec:   interfaces.GenVector{10: 1, 11: 2, 12: 3, 13: 4},
			initVec:   interfaces.GenVector{},
			genDiffIn: make(genRangeVector),
			genDiffWant: genRangeVector{
				10: &genRange{min: 1, max: 1},
				11: &genRange{min: 1, max: 2},
				12: &genRange{min: 1, max: 3},
				13: &genRange{min: 1, max: 4},
			},
		},
		{ // initiator has no updates, pre-existing diff.
			respVec: interfaces.GenVector{10: 1, 11: 2, 12: 3, 13: 4},
			initVec: interfaces.GenVector{13: 1},
			genDiffIn: genRangeVector{
				10: &genRange{min: 5, max: 20},
				13: &genRange{min: 1, max: 3},
			},
			genDiffWant: genRangeVector{
				10: &genRange{min: 1, max: 20},
				11: &genRange{min: 1, max: 2},
				12: &genRange{min: 1, max: 3},
				13: &genRange{min: 1, max: 4},
			},
		},
	}

	for _, test := range tests {
		want := test.genDiffWant
		got := test.genDiffIn
		rSt, err := newResponderState(nil, nil, s, interfaces.DeltaReqData{}, "fakeInitiator")
		if err != nil {
			t.Fatalf("newResponderState failed with err %v", err)
		}
		rSt.diff = got
		rSt.diffGenVectors(test.respVec, test.initVec)
		checkEqualDevRanges(t, got, want)
	}
}

// TestSendDeltas tests the computation of the delta bound (computeDeltaBound)
// and if the log records on the wire are correctly ordered (phases 2 and 3 of
// SendDeltas).
func TestSendDataDeltas(t *testing.T) {
	tests := []struct {
		respVecs, initVecs, outVecs interfaces.Knowledge
		respGen                     uint64
		genDiff                     genRangeVector
		keyPfxs                     []string
	}{
		{ // Identical prefixes, local and remote updates.
			respVecs: interfaces.Knowledge{
				"foo":    interfaces.GenVector{12: 8},
				"foobar": interfaces.GenVector{12: 10},
			},
			initVecs: interfaces.Knowledge{
				"foo":    interfaces.GenVector{11: 5},
				"foobar": interfaces.GenVector{11: 5},
			},
			respGen: 5,
			outVecs: interfaces.Knowledge{
				"foo":    interfaces.GenVector{10: 5, 12: 8},
				"foobar": interfaces.GenVector{10: 5, 12: 10},
			},
			genDiff: genRangeVector{
				10: &genRange{min: 1, max: 5},
				12: &genRange{min: 1, max: 10},
			},
			keyPfxs: []string{"baz", "wombat", "f", "foo", "foobar", ""},
		},
		{ // Identical prefixes, local and remote updates.
			respVecs: interfaces.Knowledge{
				"bar":    interfaces.GenVector{12: 20},
				"foo":    interfaces.GenVector{12: 8},
				"foobar": interfaces.GenVector{12: 10},
			},
			initVecs: interfaces.Knowledge{
				"foo":    interfaces.GenVector{11: 5},
				"foobar": interfaces.GenVector{11: 5, 12: 10},
				"bar":    interfaces.GenVector{10: 5, 11: 5, 12: 5},
			},
			respGen: 5,
			outVecs: interfaces.Knowledge{
				"foo":    interfaces.GenVector{10: 5, 12: 8},
				"foobar": interfaces.GenVector{10: 5, 12: 10},
				"bar":    interfaces.GenVector{10: 5, 12: 20},
			},
			genDiff: genRangeVector{
				10: &genRange{min: 1, max: 5},
				12: &genRange{min: 1, max: 20},
			},
			keyPfxs: []string{"baz", "wombat", "f", "foo", "foobar", "bar", "barbaz", ""},
		},
		{ // Non-identical prefixes, local only updates.
			initVecs: interfaces.Knowledge{
				"foo":    interfaces.GenVector{11: 5},
				"foobar": interfaces.GenVector{11: 5},
			},
			respGen: 5,
			outVecs: interfaces.Knowledge{
				"foo": interfaces.GenVector{10: 5},
			},
			genDiff: genRangeVector{
				10: &genRange{min: 1, max: 5},
			},
			keyPfxs: []string{"baz", "wombat", "f", "foo", "foobar", "", "fo", "fooxyz"},
		},
		{ // Non-identical prefixes, local and remote updates.
			respVecs: interfaces.Knowledge{
				"f":      interfaces.GenVector{12: 5, 13: 5},
				"foo":    interfaces.GenVector{12: 10, 13: 10},
				"foobar": interfaces.GenVector{12: 20, 13: 20},
			},
			initVecs: interfaces.Knowledge{
				"foo": interfaces.GenVector{11: 5, 12: 1},
			},
			respGen: 5,
			outVecs: interfaces.Knowledge{
				"foo":    interfaces.GenVector{10: 5, 12: 10, 13: 10},
				"foobar": interfaces.GenVector{10: 5, 12: 20, 13: 20},
			},
			genDiff: genRangeVector{
				10: &genRange{min: 1, max: 5},
				12: &genRange{min: 2, max: 20},
				13: &genRange{min: 1, max: 20},
			},
			keyPfxs: []string{"baz", "wombat", "f", "foo", "foobar", "", "fo", "fooxyz"},
		},
		{ // Non-identical prefixes, local and remote updates.
			respVecs: interfaces.Knowledge{
				"foobar": interfaces.GenVector{12: 20, 13: 20},
			},
			initVecs: interfaces.Knowledge{
				"foo": interfaces.GenVector{11: 5, 12: 1},
			},
			respGen: 5,
			outVecs: interfaces.Knowledge{
				"foo":    interfaces.GenVector{10: 5},
				"foobar": interfaces.GenVector{10: 5, 12: 20, 13: 20},
			},
			genDiff: genRangeVector{
				10: &genRange{min: 1, max: 5},
				12: &genRange{min: 2, max: 20},
				13: &genRange{min: 1, max: 20},
			},
			keyPfxs: []string{"baz", "wombat", "f", "foo", "foobar", "", "fo", "fooxyz"},
		},
		{ // Non-identical prefixes, local and remote updates.
			respVecs: interfaces.Knowledge{
				"f": interfaces.GenVector{12: 20, 13: 20},
			},
			initVecs: interfaces.Knowledge{
				"foo": interfaces.GenVector{11: 5, 12: 1},
			},
			respGen: 5,
			outVecs: interfaces.Knowledge{
				"foo": interfaces.GenVector{10: 5, 12: 20, 13: 20},
			},
			genDiff: genRangeVector{
				10: &genRange{min: 1, max: 5},
				12: &genRange{min: 2, max: 20},
				13: &genRange{min: 1, max: 20},
			},
			keyPfxs: []string{"baz", "wombat", "f", "foo", "foobar", "", "fo", "fooxyz"},
		},
		{ // Non-identical interleaving prefixes.
			respVecs: interfaces.Knowledge{
				"f":      interfaces.GenVector{12: 20, 13: 10},
				"foo":    interfaces.GenVector{12: 30, 13: 20},
				"foobar": interfaces.GenVector{12: 40, 13: 30},
			},
			initVecs: interfaces.Knowledge{
				"fo":        interfaces.GenVector{11: 5, 12: 1},
				"foob":      interfaces.GenVector{11: 5, 12: 10},
				"foobarxyz": interfaces.GenVector{11: 5, 12: 20},
			},
			respGen: 5,
			outVecs: interfaces.Knowledge{
				"fo":     interfaces.GenVector{10: 5, 12: 20, 13: 10},
				"foo":    interfaces.GenVector{10: 5, 12: 30, 13: 20},
				"foobar": interfaces.GenVector{10: 5, 12: 40, 13: 30},
			},
			genDiff: genRangeVector{
				10: &genRange{min: 1, max: 5},
				12: &genRange{min: 2, max: 40},
				13: &genRange{min: 1, max: 30},
			},
			keyPfxs: []string{"baz", "wombat", "f", "foo", "foobar", "", "fo", "foob", "foobarxyz", "fooxyz"},
		},
		{ // Non-identical interleaving prefixes.
			respVecs: interfaces.Knowledge{
				"fo":        interfaces.GenVector{12: 20, 13: 10},
				"foob":      interfaces.GenVector{12: 30, 13: 20},
				"foobarxyz": interfaces.GenVector{12: 40, 13: 30},
			},
			initVecs: interfaces.Knowledge{
				"f":      interfaces.GenVector{11: 5, 12: 1},
				"foo":    interfaces.GenVector{11: 5, 12: 10},
				"foobar": interfaces.GenVector{11: 5, 12: 20},
			},
			respGen: 5,
			outVecs: interfaces.Knowledge{
				"f":         interfaces.GenVector{10: 5},
				"fo":        interfaces.GenVector{10: 5, 12: 20, 13: 10},
				"foob":      interfaces.GenVector{10: 5, 12: 30, 13: 20},
				"foobarxyz": interfaces.GenVector{10: 5, 12: 40, 13: 30},
			},
			genDiff: genRangeVector{
				10: &genRange{min: 1, max: 5},
				12: &genRange{min: 2, max: 40},
				13: &genRange{min: 1, max: 30},
			},
			keyPfxs: []string{"baz", "wombat", "f", "foo", "foobar", "", "fo", "foob", "foobarxyz", "fooxyz"},
		},
		{ // Non-identical sibling prefixes.
			respVecs: interfaces.Knowledge{
				"foo":       interfaces.GenVector{12: 20, 13: 10},
				"foobarabc": interfaces.GenVector{12: 40, 13: 30},
				"foobarxyz": interfaces.GenVector{12: 30, 13: 20},
			},
			initVecs: interfaces.Knowledge{
				"foo": interfaces.GenVector{11: 5, 12: 1},
			},
			respGen: 5,
			outVecs: interfaces.Knowledge{
				"foo":       interfaces.GenVector{10: 5, 12: 20, 13: 10},
				"foobarabc": interfaces.GenVector{10: 5, 12: 40, 13: 30},
				"foobarxyz": interfaces.GenVector{10: 5, 12: 30, 13: 20},
			},
			genDiff: genRangeVector{
				10: &genRange{min: 1, max: 5},
				12: &genRange{min: 2, max: 40},
				13: &genRange{min: 1, max: 30},
			},
			keyPfxs: []string{"baz", "wombat", "f", "foo", "foobar", "", "foobarabc", "foobarxyz", "foobar123", "fooxyz"},
		},
		{ // Non-identical prefixes, local and remote updates.
			respVecs: interfaces.Knowledge{
				"barbaz": interfaces.GenVector{12: 18},
				"f":      interfaces.GenVector{12: 30, 13: 5},
				"foobar": interfaces.GenVector{12: 30, 13: 8},
			},
			initVecs: interfaces.Knowledge{
				"foo":    interfaces.GenVector{11: 5, 12: 5},
				"foobar": interfaces.GenVector{11: 5, 12: 5},
				"bar":    interfaces.GenVector{10: 5, 11: 5, 12: 5},
			},
			respGen: 5,
			outVecs: interfaces.Knowledge{
				"foo":    interfaces.GenVector{10: 5, 12: 30, 13: 5},
				"foobar": interfaces.GenVector{10: 5, 12: 30, 13: 8},
				"bar":    interfaces.GenVector{10: 5},
				"barbaz": interfaces.GenVector{10: 5, 12: 18},
			},
			genDiff: genRangeVector{
				10: &genRange{min: 1, max: 5},
				12: &genRange{min: 6, max: 30},
				13: &genRange{min: 1, max: 8},
			},
			keyPfxs: []string{"baz", "wombat", "f", "foo", "foobar", "bar", "barbaz", ""},
		},
	}

	for i, test := range tests {
		svc := createService(t)
		s := svc.sync
		s.id = 10 //responder.

		wantDiff, wantVecs := test.genDiff, test.outVecs
		s.syncState[mockDbId] = &dbSyncStateInMem{
			data: &localGenInfoInMem{
				gen:        test.respGen,
				checkptGen: test.respGen,
			},
			genvecs: test.respVecs,
		}

		////////////////////////////////////////
		// Test sending deltas.

		// Insert some log records to bootstrap testing below.
		tRng := rand.New(rand.NewSource(int64(i)))
		var wantRecs []*LocalLogRec
		tx := createDatabase(t, svc).St().NewWatchableTransaction()
		objKeyPfxs := test.keyPfxs
		j := 0
		for id, r := range wantDiff {
			pos := uint64(tRng.Intn(50) + 100*j)
			for k := r.min; k <= r.max; k++ {
				opfx := objKeyPfxs[tRng.Intn(len(objKeyPfxs))]
				// Create holes in the log records.
				if opfx == "" {
					continue
				}
				okey := makeRowKey(fmt.Sprintf("%s~%x", opfx, tRng.Int()))
				vers := fmt.Sprintf("%x", tRng.Int())
				rec := &LocalLogRec{
					Metadata: interfaces.LogRecMetadata{Id: id, Gen: k, ObjId: okey, CurVers: vers, UpdTime: time.Now().UTC()},
					Pos:      pos + k,
				}
				if err := putLogRec(nil, tx, logDataPrefix, rec); err != nil {
					t.Fatalf("putLogRec(%d:%d) failed rec %v err %v", id, k, rec, err)
				}
				value := fmt.Sprintf("value_%s", okey)
				var encodedValue []byte
				var err error
				if encodedValue, err = vom.Encode(value); err != nil {
					t.Fatalf("vom.Encode(%q) failed err %v", value, err)
				}
				if err := watchable.PutAtVersion(nil, tx, []byte(okey), encodedValue, []byte(vers)); err != nil {
					t.Fatalf("PutAtVersion(%d:%d) failed rec %v value %s: err %v", id, k, rec, value, err)
				}

				initPfxs := extractAndSortPrefixes(test.initVecs)
				if !filterLogRec(rec, test.initVecs, initPfxs) {
					wantRecs = append(wantRecs, rec)
				}
			}
			j++
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("cannot commit putting log rec, err %v", err)
		}

		req := interfaces.DataDeltaReq{
			DbId: mockDbId,
			Gvs:  test.initVecs,
		}

		rSt, err := newResponderState(nil, nil, s, interfaces.DeltaReqData{req}, "fakeInitiator")
		if err != nil {
			t.Fatalf("newResponderState failed with err %v", err)
		}
		d := &dummyResponder{}
		rSt.call = d
		rSt.st, err = rSt.sync.getDbStore(nil, nil, rSt.dbId)
		if err != nil {
			t.Fatalf("getDbStore failed to get store handle for db %v", rSt.dbId)
		}

		err = rSt.computeDataDeltas(nil)
		if err != nil || !reflect.DeepEqual(rSt.outVecs, wantVecs) {
			t.Fatalf("computeDataDeltas failed (I: %v), (R: %v, %v), got %v, want %v err %v", test.initVecs, test.respGen, test.respVecs, rSt.outVecs, wantVecs, err)
		}
		checkEqualDevRanges(t, rSt.diff, wantDiff)

		if err = rSt.sendDataDeltas(nil); err != nil {
			t.Fatalf("sendDataDeltas failed, err %v", err)
		}

		d.diffLogRecs(t, wantRecs, wantVecs)

		destroyService(t, svc)
	}
}

//////////////////////////////
// Helpers

type dummyResponder struct {
	gotRecs []*LocalLogRec
	outVecs interfaces.Knowledge
}

func (d *dummyResponder) SendStream() interface {
	Send(item interfaces.DeltaResp) error
} {
	return d
}

func (d *dummyResponder) Send(item interfaces.DeltaResp) error {
	switch v := item.(type) {
	case interfaces.DeltaRespGvs:
		d.outVecs = v.Value
	case interfaces.DeltaRespRec:
		d.gotRecs = append(d.gotRecs, &LocalLogRec{Metadata: v.Value.Metadata})
	}
	return nil
}

func (d *dummyResponder) Security() security.Call {
	return nil
}

func (d *dummyResponder) Suffix() string {
	return ""
}

func (d *dummyResponder) LocalEndpoint() naming.Endpoint {
	return naming.Endpoint{}
}

func (d *dummyResponder) RemoteEndpoint() naming.Endpoint {
	return naming.Endpoint{}
}

func (d *dummyResponder) GrantedBlessings() security.Blessings {
	return security.Blessings{}
}

func (d *dummyResponder) Server() rpc.Server {
	return nil
}

func (d *dummyResponder) diffLogRecs(t *testing.T, wantRecs []*LocalLogRec, wantVecs interfaces.Knowledge) {
	if len(d.gotRecs) != len(wantRecs) {
		t.Fatalf("diffLogRecs failed, gotLen %v, wantLen %v\n", len(d.gotRecs), len(wantRecs))
	}
	for i, rec := range d.gotRecs {
		if !reflect.DeepEqual(rec.Metadata, wantRecs[i].Metadata) {
			t.Fatalf("diffLogRecs failed, i %v, got %v, want %v\n", i, rec.Metadata, wantRecs[i].Metadata)
		}
	}
	if !reflect.DeepEqual(d.outVecs, wantVecs) {
		t.Fatalf("diffLogRecs failed genvector, got %v, want %v\n", d.outVecs, wantVecs)
	}
}

func checkEqualDevRanges(t *testing.T, s1, s2 genRangeVector) {
	if len(s1) != len(s2) {
		t.Fatalf("len(s1): %v != len(s2): %v", len(s1), len(s2))
	}
	for d1, r1 := range s1 {
		if r2, ok := s2[d1]; !ok || !reflect.DeepEqual(r1, r2) {
			t.Fatalf("Dev %v: r1 %v != r2 %v", d1, r1, r2)
		}
	}
}

// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sbtree_test

import (
	"reflect"
	"testing"

	"v.io/v23/syncbase"
	"v.io/v23/vdl"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/debug/debug/browseserver/sbtree"
	tu "v.io/x/ref/services/syncbase/testutil"
)

func TestEmptyCollection(t *testing.T) {
	ctx, serverName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	var (
		service = syncbase.NewService(serverName)
		db      = tu.CreateDatabase(t, ctx, service, "the_db")
		coll    = tu.CreateCollection(t, ctx, db, "the_collection")
	)

	got := sbtree.AssembleCollectionTree(
		ctx,
		serverName,
		db.Id().Blessing,
		"the_db",
		coll.Id().Blessing,
		"the_collection",
		"",
		100,
	)

	if got.Service.FullName() != serverName {
		t.Errorf("got %q, want %q", got.Service.FullName(), serverName)
	}
	if got.Database.Id().Name != "the_db" {
		t.Errorf(`got %q, want "the_db"`, got.Database.Id().Name)
	}
	if got.Collection.Id().Name != "the_collection" {
		t.Errorf(`got %q, want "the_collection"`, got.Database.Id().Name)
	}
	if got.RowCount != 0 {
		t.Errorf("got %d rows, want none", got.RowCount)
	}
	if got.TotKeySize != 0 {
		t.Errorf("got %d bytes of keys, want zero", got.TotKeySize)
	}
	if got.KeysPage.HasPrev {
		t.Error("Got previous page, want no previous page")
	}
	if len(got.KeysPage.KeyVals) != 0 {
		t.Errorf("Want no keys, got %v (length %d)",
			got.KeysPage.KeyVals, len(got.KeysPage.KeyVals))
	}
	if got.KeysPage.NextKey != "" {
		t.Errorf("Got %q, want empty string", got.KeysPage.NextKey)
	}
}

func TestSingleKeysPage(t *testing.T) {
	ctx, serverName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	var (
		service        = syncbase.NewService(serverName)
		db             = tu.CreateDatabase(t, ctx, service, "the_db")
		coll           = tu.CreateCollection(t, ctx, db, "the_collection")
		wantTotKeySize = len("Bravo") + len("Alfa") + len("Delta") + len("Charlie")
	)
	// Put keys of sundry types in non-alphabethical order
	coll.Put(ctx, "Bravo", int64(9999))
	coll.Put(ctx, "Alfa", complex(11, 22))
	coll.Put(ctx, "Delta", "something")
	coll.Put(ctx, "Charlie", 'x')

	got := sbtree.AssembleCollectionTree(
		ctx,
		serverName,
		db.Id().Blessing,
		"the_db",
		coll.Id().Blessing,
		"the_collection",
		"",
		100,
	)

	if got.RowCount != 4 {
		t.Errorf("got %d rows, want 4", got.RowCount)
	}
	if int(got.TotKeySize) != wantTotKeySize {
		t.Errorf("got %d bytes of keys, want %d", got.TotKeySize, wantTotKeySize)
	}
	if got.KeysPage.HasPrev {
		t.Error("Got previous page, want no previous page")
	}
	if len(got.KeysPage.KeyVals) != 4 {
		t.Errorf("Wanted 4 keys, got %v (length %d)",
			got.KeysPage.KeyVals, len(got.KeysPage.KeyVals))
	}
	for i := 0; i < 4; i++ {
		if got.KeysPage.KeyVals[i].Index != i {
			t.Errorf("got %d, want %d", got.KeysPage.KeyVals[i].Index, i)
		}
	}
	// Make sure keys come back in alphabetical order
	for i, want := range []string{"Alfa", "Bravo", "Charlie", "Delta"} {
		if got.KeysPage.KeyVals[i].Key != want {
			t.Errorf("got %q, want %q", got.KeysPage.KeyVals[i].Key, want)
		}
	}
	for i, want := range []interface{}{complex(11, 22), int64(9999), 'x', "something"} {
		if val := vdl.ValueOf(want); !reflect.DeepEqual(got.KeysPage.KeyVals[i].Value, val) {
			t.Errorf("got %v of type %T, want %v of type %T",
				got.KeysPage.KeyVals[i].Value, got.KeysPage.KeyVals[i].Value, val, val)
		}
	}

	if got.KeysPage.NextKey != "" {
		t.Errorf("Got %q, want empty string", got.KeysPage.NextKey)
	}
}

func TestFirstOfMultiplePages(t *testing.T) {
	ctx, serverName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	var (
		service = syncbase.NewService(serverName)
		db      = tu.CreateDatabase(t, ctx, service, "the_db")
		coll    = tu.CreateCollection(t, ctx, db, "the_collection")
	)
	// Ten keys, in pages of four, starting with the first key
	coll.Put(ctx, "555", 99)
	coll.Put(ctx, "000", 99)
	coll.Put(ctx, "999", 99)
	coll.Put(ctx, "111", 99)
	coll.Put(ctx, "444", 99)
	coll.Put(ctx, "666", 99)
	coll.Put(ctx, "222", 99)
	coll.Put(ctx, "777", 99)
	coll.Put(ctx, "333", 99)
	coll.Put(ctx, "888", 99)
	got := sbtree.AssembleCollectionTree(
		ctx,
		serverName,
		db.Id().Blessing,
		"the_db",
		coll.Id().Blessing,
		"the_collection",
		"",
		4,
	)

	if got.RowCount != 10 {
		t.Errorf("got %d rows, want 10", got.RowCount)
	}
	if int(got.TotKeySize) != 30 { // 10 keys, each of length 3
		t.Errorf("got %d bytes of keys, want 30", got.TotKeySize)
	}
	if got.KeysPage.HasPrev {
		t.Error("Got previous page, want no previous page")
	}
	if len(got.KeysPage.KeyVals) != 4 {
		t.Errorf("Wanted 4 keys, got %v (length %d)",
			got.KeysPage.KeyVals, len(got.KeysPage.KeyVals))
	}
	for i := 0; i < 4; i++ {
		if got.KeysPage.KeyVals[i].Index != i {
			t.Errorf("got %d, want %d", got.KeysPage.KeyVals[i].Index, i)
		}
	}
	for i, want := range []string{"000", "111", "222", "333"} {
		if got.KeysPage.KeyVals[i].Key != want {
			t.Errorf("got %q, want %q", got.KeysPage.KeyVals[i].Key, want)
		}
	}
	if got.KeysPage.NextKey != "444" {
		t.Errorf(`Got %q, want "444"`, got.KeysPage.NextKey)
	}
}

func TestMiddleOfMultiplePages(t *testing.T) {
	ctx, serverName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	var (
		service = syncbase.NewService(serverName)
		db      = tu.CreateDatabase(t, ctx, service, "the_db")
		coll    = tu.CreateCollection(t, ctx, db, "the_collection")
	)
	// Ten keys, in pages of four, starting with fifth key
	coll.Put(ctx, "555", 99)
	coll.Put(ctx, "000", 99)
	coll.Put(ctx, "999", 99)
	coll.Put(ctx, "111", 99)
	coll.Put(ctx, "444", 99)
	coll.Put(ctx, "666", 99)
	coll.Put(ctx, "222", 99)
	coll.Put(ctx, "777", 99)
	coll.Put(ctx, "333", 99)
	coll.Put(ctx, "888", 99)
	got := sbtree.AssembleCollectionTree(
		ctx,
		serverName,
		db.Id().Blessing,
		"the_db",
		coll.Id().Blessing,
		"the_collection",
		"444",
		4,
	)

	if got.RowCount != 10 {
		t.Errorf("got %d rows, want 10", got.RowCount)
	}
	if int(got.TotKeySize) != 30 { // 10 keys, each of length 3
		t.Errorf("got %d bytes of keys, want 30", got.TotKeySize)
	}
	if !got.KeysPage.HasPrev {
		t.Error("No previous page, want previous page")
	}
	if len(got.KeysPage.KeyVals) != 4 {
		t.Errorf("Wanted 4 keys, got %v (length %d)",
			got.KeysPage.KeyVals, len(got.KeysPage.KeyVals))
	}
	for i := 0; i < 4; i++ {
		if got.KeysPage.KeyVals[i].Index != i+4 {
			t.Errorf("got %d, want %d", got.KeysPage.KeyVals[i].Index, i+4)
		}
	}
	for i, want := range []string{"444", "555", "666", "777"} {
		if got.KeysPage.KeyVals[i].Key != want {
			t.Errorf("got %q, want %q", got.KeysPage.KeyVals[i].Key, want)
		}
	}
	if got.KeysPage.NextKey != "888" {
		t.Errorf(`Got %q, want "888"`, got.KeysPage.NextKey)
	}
}

func TestLastOfMultiplePages(t *testing.T) {
	ctx, serverName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	var (
		service = syncbase.NewService(serverName)
		db      = tu.CreateDatabase(t, ctx, service, "the_db")
		coll    = tu.CreateCollection(t, ctx, db, "the_collection")
	)
	// Ten keys, in pages of four, starting with ninth key.
	coll.Put(ctx, "555", 99)
	coll.Put(ctx, "000", 99)
	coll.Put(ctx, "999", 99)
	coll.Put(ctx, "111", 99)
	coll.Put(ctx, "444", 99)
	coll.Put(ctx, "666", 99)
	coll.Put(ctx, "222", 99)
	coll.Put(ctx, "777", 99)
	coll.Put(ctx, "333", 99)
	coll.Put(ctx, "888", 99)
	got := sbtree.AssembleCollectionTree(
		ctx,
		serverName,
		db.Id().Blessing,
		"the_db",
		coll.Id().Blessing,
		"the_collection",
		"888",
		4,
	)

	if got.RowCount != 10 {
		t.Errorf("got %d rows, want 10", got.RowCount)
	}
	if int(got.TotKeySize) != 30 { // 10 keys, each of length 3
		t.Errorf("got %d bytes of keys, want 30", got.TotKeySize)
	}
	if !got.KeysPage.HasPrev {
		t.Error("No previous page, want previous page")
	}
	if len(got.KeysPage.KeyVals) != 2 {
		t.Errorf("Wanted 2 keys, got %v (length %d)",
			got.KeysPage.KeyVals, len(got.KeysPage.KeyVals))
	}
	for i := 0; i < 2; i++ {
		if got.KeysPage.KeyVals[i].Index != i+8 {
			t.Errorf("got %d, want %d", got.KeysPage.KeyVals[i].Index, i+8)
		}
	}
	// Make sure keys come back in alphabetical order.
	for i, want := range []string{"888", "999"} {
		if got.KeysPage.KeyVals[i].Key != want {
			t.Errorf("got %q, want %q", got.KeysPage.KeyVals[i].Key, want)
		}
	}
	if got.KeysPage.NextKey != "" {
		t.Errorf(`Got %q, want empty string`, got.KeysPage.NextKey)
	}
}

func TestNonBuiltInType(t *testing.T) {
	ctx, serverName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	var (
		service = syncbase.NewService(serverName)
		db      = tu.CreateDatabase(t, ctx, service, "the_db")
		coll    = tu.CreateCollection(t, ctx, db, "the_collection")
	)
	type childType struct {
		I int64
		C complex128
	}
	type someCustomType struct {
		S string
		R rune
		C childType
	}
	err := coll.Put(ctx, "the key",
		someCustomType{"something", 'x', childType{9999, complex(11, 22)}})
	if err != nil {
		t.Fatal(err)
	}

	got := sbtree.AssembleCollectionTree(
		ctx,
		serverName,
		db.Id().Blessing,
		"the_db",
		coll.Id().Blessing,
		"the_collection",
		"",
		100,
	)

	if got.RowCount != 1 {
		t.Fatalf("got %d rows, want 1", got.RowCount)
	}
	if len(got.KeysPage.KeyVals) != 1 {
		t.Errorf("Wanted 1 keys, got %v (length %d)",
			got.KeysPage.KeyVals, len(got.KeysPage.KeyVals))
	}
	value := got.KeysPage.KeyVals[0].Value.(*vdl.Value)
	if value.Type() != vdl.TypeOf(someCustomType{}) {
		t.Fatalf("Got %v of type %T, want of type someCustomType",
			value, value.Type)
	}
	want := vdl.ValueOf(someCustomType{"something", 'x', childType{9999, complex(11, 22)}})
	if !reflect.DeepEqual(value, want) {
		t.Errorf("Got %v, want %v", value, want)
	}
}

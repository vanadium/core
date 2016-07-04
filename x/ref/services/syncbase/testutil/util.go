// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testutil

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"runtime/debug"
	"strings"
	"testing"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/services/watch"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/util"
	"v.io/v23/vdl"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/x/lib/vlog"
	idiscovery "v.io/x/ref/lib/discovery"
	fdiscovery "v.io/x/ref/lib/discovery/factory"
	"v.io/x/ref/lib/discovery/plugins/mock"
	"v.io/x/ref/services/syncbase/server"
	"v.io/x/ref/services/syncbase/store"
	"v.io/x/ref/test"
	tsecurity "v.io/x/ref/test/testutil"
)

func Fatal(t testing.TB, args ...interface{}) {
	debug.PrintStack()
	t.Fatal(args...)
}

func Fatalf(t testing.TB, format string, args ...interface{}) {
	debug.PrintStack()
	t.Fatalf(format, args...)
}

// TODO(sadovsky): Standardize on a small set of constants and helper functions
// to share across all Syncbase tests. Currently, our 'featuretests' tests use a
// different set of helpers from our other unit tests.
func CreateDatabase(t testing.TB, ctx *context.T, s syncbase.Service, name string) syncbase.Database {
	d := s.Database(ctx, name, nil)
	if err := d.Create(ctx, nil); err != nil {
		Fatalf(t, "d.Create() failed: %v", err)
	}
	return d
}

func CreateCollection(t testing.TB, ctx *context.T, d syncbase.Database, name string) syncbase.Collection {
	c := d.Collection(ctx, name)
	if err := c.Create(ctx, nil); err != nil {
		Fatalf(t, "c.Create() failed: %v", err)
	}
	return c
}

func CreateSyncgroup(t testing.TB, ctx *context.T, d syncbase.Database, c syncbase.Collection, name, description string) syncbase.Syncgroup {
	sg := d.Syncgroup(ctx, name)
	sgSpec := wire.SyncgroupSpec{
		Description: description,
		Collections: []wire.Id{c.Id()},
		Perms:       DefaultPerms(wire.AllSyncgroupTags, string(security.AllPrincipals)),
	}
	sgMembership := wire.SyncgroupMemberInfo{}
	if err := sg.Create(ctx, sgSpec, sgMembership); err != nil {
		Fatalf(t, "sg.Create() failed: %v", err)
	}
	return sg
}

// TODO(sadovsky): Drop the 'perms' argument. The only client that passes
// non-nil, syncgroup_test.go, should use SetupOrDieCustom instead.
func SetupOrDie(perms access.Permissions) (clientCtx *context.T, serverName string, cleanup func()) {
	_, clientCtx, serverName, _, cleanup = SetupOrDieCustom(strings.Join([]string{"o", "app", "client"}, security.ChainSeparator), "server", perms)
	return
}

// TODO(sadovsky): Switch unit tests to v23test.Shell, then delete this.
func SetupOrDieCustom(clientSuffix, serverSuffix string, perms access.Permissions) (ctx, clientCtx *context.T, serverName string, rootp security.Principal, cleanup func()) {
	ctx, shutdown := test.V23Init()
	df, _ := idiscovery.NewFactory(ctx, mock.New())
	fdiscovery.InjectFactory(df)
	rootp = tsecurity.NewPrincipal("root")
	clientCtx, serverCtx := NewCtx(ctx, rootp, clientSuffix), NewCtx(ctx, rootp, serverSuffix)

	if perms == nil {
		perms = DefaultPerms(access.AllTypicalTags(), "root"+security.ChainSeparator+clientSuffix)
	}
	serverName, stopServer := newServer(serverCtx, perms)
	cleanup = func() {
		stopServer()
		shutdown()
	}
	return
}

func DefaultPerms(tags []access.Tag, patterns ...string) access.Permissions {
	perms := access.Permissions{}
	for _, pattern := range patterns {
		perms.Add(security.BlessingPattern(pattern), access.TagStrings(tags...)...)
	}
	return perms
}

func ScanMatches(ctx *context.T, c syncbase.Collection, r syncbase.RowRange, wantKeys []string, wantValues []interface{}) error {
	if len(wantKeys) != len(wantValues) {
		return fmt.Errorf("bad input args")
	}
	it := c.Scan(ctx, r)
	gotKeys := []string{}
	for it.Advance() {
		gotKey := it.Key()
		gotKeys = append(gotKeys, gotKey)
		i := len(gotKeys) - 1
		if i >= len(wantKeys) {
			continue
		}
		// Check key.
		wantKey := wantKeys[i]
		if gotKey != wantKey {
			return fmt.Errorf("Keys do not match: got %q, want %q", gotKey, wantKey)
		}
		// Check value.
		wantValue := wantValues[i]
		gotValue := reflect.Zero(reflect.TypeOf(wantValue)).Interface()
		if err := it.Value(&gotValue); err != nil {
			return fmt.Errorf("it.Value() failed: %v", err)
		}
		if !reflect.DeepEqual(gotValue, wantValue) {
			return fmt.Errorf("Values do not match: got %v, want %v", gotValue, wantValue)
		}
	}
	if err := it.Err(); err != nil {
		return fmt.Errorf("c.Scan() failed: %v", err)
	}
	if len(gotKeys) != len(wantKeys) {
		return fmt.Errorf("Unmatched keys: got %v, want %v", gotKeys, wantKeys)
	}
	return nil
}

func CheckScan(t testing.TB, ctx *context.T, c syncbase.Collection, r syncbase.RowRange, wantKeys []string, wantValues []interface{}) {
	if err := ScanMatches(ctx, c, r, wantKeys, wantValues); err != nil {
		Fatalf(t, err.Error())
	}
}

func CheckExec(t testing.TB, ctx *context.T, db syncbase.DatabaseHandle, q string, wantHeaders []string, wantResults [][]*vom.RawBytes) {
	gotHeaders, it, err := db.Exec(ctx, q)
	if err != nil {
		t.Errorf("query %q: got %v, want nil", q, err)
	}
	if !reflect.DeepEqual(gotHeaders, wantHeaders) {
		t.Errorf("query %q: got %v, want %v", q, gotHeaders, wantHeaders)
	}
	gotResults := [][]interface{}{}
	for it.Advance() {
		n := it.ResultCount()
		gotResult := make([]interface{}, n)
		for i := 0; i != n; i++ {
			if err := it.Result(i, &gotResult[i]); err != nil {
				t.Errorf("error decoding result: %v", err)
			}
		}
		gotResults = append(gotResults, gotResult)
	}
	if it.Err() != nil {
		t.Errorf("query %q: got %v, want nil", q, it.Err())
	}
	if got, want := vdl.ValueOf(gotResults), vdl.ValueOf(wantResults); !vdl.EqualValue(got, want) {
		t.Errorf("query %q: got %v, want %v", q, got, want)
	}
}

func CheckExecError(t testing.TB, ctx *context.T, db syncbase.DatabaseHandle, q string, wantErrorID verror.ID) {
	_, rs, err := db.Exec(ctx, q)
	if err == nil {
		if rs.Advance() {
			t.Errorf("query %q: got true, want false", q)
		}
		err = rs.Err()
	}
	if verror.ErrorID(err) != wantErrorID {
		t.Errorf("%q", verror.DebugString(err))
		t.Errorf("query %q: got %v, want: %v", q, verror.ErrorID(err), wantErrorID)
	}
}

// A WatchChangeTest is a syncbase.WatchChange that has a public ValueBytes
// and CollectionInfo field, to allow tests to set them.
type WatchChangeTest struct {
	syncbase.WatchChange
	ValueBytes     *vom.RawBytes
	CollectionInfo *wire.StoreChangeCollectionInfo
}

func WatchChangeTestRootPut(resumeMarker watch.ResumeMarker) WatchChangeTest {
	return WatchChangeTest{
		WatchChange: syncbase.WatchChange{
			EntityType:   syncbase.EntityRoot,
			ChangeType:   syncbase.PutChange,
			ResumeMarker: resumeMarker,
			Continued:    (resumeMarker == nil),
		},
	}
}

func WatchChangeTestCollectionPut(cxId wire.Id, allowedTags []access.Tag, perms access.Permissions, resumeMarker watch.ResumeMarker) WatchChangeTest {
	allowedTagsSet := make(map[access.Tag]struct{})
	if len(allowedTags) == 0 {
		allowedTagsSet = nil
	} else {
		for _, tag := range allowedTags {
			allowedTagsSet[tag] = struct{}{}
		}
	}
	return WatchChangeTest{
		WatchChange: syncbase.WatchChange{
			EntityType:   syncbase.EntityCollection,
			Collection:   cxId,
			ChangeType:   syncbase.PutChange,
			ResumeMarker: resumeMarker,
			Continued:    (resumeMarker == nil),
		},
		CollectionInfo: &wire.StoreChangeCollectionInfo{
			Allowed: allowedTagsSet,
			Perms:   perms,
		},
	}
}

func WatchChangeTestCollectionDelete(cxId wire.Id, resumeMarker watch.ResumeMarker) WatchChangeTest {
	return WatchChangeTest{
		WatchChange: syncbase.WatchChange{
			EntityType:   syncbase.EntityCollection,
			Collection:   cxId,
			ChangeType:   syncbase.DeleteChange,
			ResumeMarker: resumeMarker,
			Continued:    (resumeMarker == nil),
		},
	}
}

func WatchChangeTestRowPut(cxId wire.Id, rowKey string, value interface{}, resumeMarker watch.ResumeMarker) WatchChangeTest {
	return WatchChangeTest{
		WatchChange: syncbase.WatchChange{
			EntityType:   syncbase.EntityRow,
			Collection:   cxId,
			Row:          rowKey,
			ChangeType:   syncbase.PutChange,
			ResumeMarker: resumeMarker,
			Continued:    (resumeMarker == nil),
		},
		ValueBytes: vom.RawBytesOf(value),
	}
}

func WatchChangeTestRowDelete(cxId wire.Id, rowKey string, resumeMarker watch.ResumeMarker) WatchChangeTest {
	return WatchChangeTest{
		WatchChange: syncbase.WatchChange{
			EntityType:   syncbase.EntityRow,
			Collection:   cxId,
			Row:          rowKey,
			ChangeType:   syncbase.DeleteChange,
			ResumeMarker: resumeMarker,
			Continued:    (resumeMarker == nil),
		},
	}
}

// WatchChangeEq returns whether *want and *got represent the same value.
func WatchChangeEq(got *syncbase.WatchChange, want *WatchChangeTest) (eq bool) {
	if want.EntityType == got.EntityType &&
		want.Collection == got.Collection &&
		want.Row == got.Row &&
		want.ChangeType == got.ChangeType &&
		bytes.Equal(want.ResumeMarker, got.ResumeMarker) &&
		want.FromSync == got.FromSync &&
		want.Continued == got.Continued {

		if want.ChangeType == syncbase.DeleteChange {
			eq = true
		} else {
			switch want.EntityType {
			case syncbase.EntityRow:
				var wantValue interface{}
				var gotValue interface{}
				gotErr := got.Value(&gotValue)
				wantErr := want.ValueBytes.ToValue(&wantValue)
				eq = ((gotErr == nil) == (wantErr == nil)) &&
					reflect.DeepEqual(gotValue, wantValue)
			case syncbase.EntityCollection:
				eq = reflect.DeepEqual(got.CollectionInfo(), want.CollectionInfo)
			default:
				eq = true
			}
		}
	}
	return eq
}

// CheckWatch checks that the sequence of elements from the watch stream starts
// with the given slice of watch changes.
func CheckWatch(t testing.TB, wstream syncbase.WatchStream, changes []WatchChangeTest) {
	for _, want := range changes {
		if !wstream.Advance() {
			Fatalf(t, "wstream.Advance() reached the end: %v", wstream.Err())
		}
		if got := wstream.Change(); !WatchChangeEq(&got, &want) {
			if got.ChangeType == syncbase.PutChange && got.EntityType == syncbase.EntityCollection {
				Fatalf(t, "unexpected watch change: got %+v, want %+v (cx info: got %+v, want %+v)", got, want, got.CollectionInfo(), want.CollectionInfo)
			} else {
				Fatalf(t, "unexpected watch change: got %+v, want %+v", got, want)
			}
		}
	}
}

func DefaultSchema(version int32) *syncbase.Schema {
	return &syncbase.Schema{
		Metadata: wire.SchemaMetadata{
			Version: version,
		},
	}
}

////////////////////////////////////////
// Internal helpers

func getPermsOrDie(t testing.TB, ctx *context.T, ac util.AccessController) access.Permissions {
	perms, _, err := ac.GetPermissions(ctx)
	if err != nil {
		Fatalf(t, "GetPermissions failed: %v", err)
	}
	return perms
}

func newServer(serverCtx *context.T, perms access.Permissions) (string, func()) {
	if perms == nil {
		vlog.Fatal("perms must be specified")
	}
	rootDir, err := ioutil.TempDir("", "syncbase")
	if err != nil {
		vlog.Fatal("ioutil.TempDir() failed: ", err)
	}
	serverCtx, cancel := context.WithCancel(serverCtx)
	service, err := server.NewService(serverCtx, server.ServiceOptions{
		Perms:   perms,
		RootDir: rootDir,
		Engine:  store.EngineForTest,
		DevMode: true,
	})
	if err != nil {
		vlog.Fatal("server.NewService() failed: ", err)
	}
	serverCtx, s, err := v23.WithNewDispatchingServer(serverCtx, "", server.NewDispatcher(service))
	if err != nil {
		vlog.Fatal("v23.WithNewDispatchingServer() failed: ", err)
	}
	service.AddNames(serverCtx, s)
	name := s.Status().Endpoints[0].Name()
	return name, func() {
		cancel()
		<-s.Closed()
		service.Close()
		os.RemoveAll(rootDir)
	}
}

// Creates a new context object with blessing "root:<suffix>", configured to
// present this blessing when acting as a server as well as when acting as a
// client and talking to a server that presents a blessing rooted at "root".
// TODO(sadovsky): Switch unit tests to v23test.Shell, then delete this.
func NewCtx(ctx *context.T, rootp security.Principal, suffix string) *context.T {
	// Principal for the new context.
	p := tsecurity.NewPrincipal(suffix)
	rootb, _ := rootp.BlessingStore().Default()

	// Bless the new principal as "root:<suffix>".
	blessings, err := rootp.Bless(p.PublicKey(), rootb, suffix, security.UnconstrainedUse())
	if err != nil {
		vlog.Fatal("rootp.Bless() failed: ", err)
	}

	// Make it so users of the new context present their "root:<suffix>" blessing
	// when talking to servers with blessings rooted at "root".
	if _, err := p.BlessingStore().Set(blessings, security.BlessingPattern("root")); err != nil {
		vlog.Fatal("p.BlessingStore().Set() failed: ", err)
	}

	// Make it so that when users of the new context act as a server, they present
	// their "root:<suffix>" blessing.
	if err := p.BlessingStore().SetDefault(blessings); err != nil {
		vlog.Fatal("p.BlessingStore().SetDefault() failed: ", err)
	}

	// Have users of the prepared context treat root's public key as an authority
	// on all blessings rooted at "root".
	if err := security.AddToRoots(p, blessings); err != nil {
		vlog.Fatal("AddToRoots() failed: ", err)
	}

	resCtx, err := v23.WithPrincipal(ctx, p)
	if err != nil {
		vlog.Fatal("v23.WithPrincipal() failed: ", err)
	}

	return resCtx
}

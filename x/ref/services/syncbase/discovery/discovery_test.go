// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package discovery_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/discovery"
	"v.io/v23/security"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase"
	"v.io/v23/verror"
	idiscovery "v.io/x/ref/lib/discovery"
	fdiscovery "v.io/x/ref/lib/discovery/factory"
	"v.io/x/ref/lib/discovery/global"
	"v.io/x/ref/lib/discovery/plugins/mock"
	udiscovery "v.io/x/ref/lib/discovery/testutil"
	_ "v.io/x/ref/runtime/factories/roaming"
	syncdis "v.io/x/ref/services/syncbase/discovery"
	"v.io/x/ref/services/syncbase/server/interfaces"
	tu "v.io/x/ref/services/syncbase/testutil"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

func TestSyncgroupDiscovery(t *testing.T) {
	_, ctx, sName, rootp, cleanup := tu.SetupOrDieCustom("o:app1:client1", "server",
		tu.DefaultPerms(access.AllTypicalTags(), "root:o:app1:client1"))
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	collection1 := tu.CreateCollection(t, ctx, d, "c1")
	collection2 := tu.CreateCollection(t, ctx, d, "c2")

	c1Updates, err := scanAs(ctx, rootp, "o:app1:client1")
	if err != nil {
		panic(err)
	}
	c2Updates, err := scanAs(ctx, rootp, "o:app1:client2")
	if err != nil {
		panic(err)
	}

	sgId := d.Syncgroup(ctx, "sg1").Id()
	spec := wire.SyncgroupSpec{
		Description: "test syncgroup sg1",
		Perms:       tu.DefaultPerms(wire.AllSyncgroupTags, "root:server", "root:o:app1:client1"),
		Collections: []wire.Id{collection1.Id()},
	}
	createSyncgroup(t, ctx, d, sgId, spec, verror.ID(""))

	// First update is for syncbase, and not a specific syncgroup.
	u := <-c1Updates
	attrs := u.Advertisement().Attributes
	peer := attrs[wire.DiscoveryAttrPeer]
	if peer == "" || len(attrs) != 1 {
		t.Errorf("Got %v, expected only a peer name.", attrs)
	}
	// Client2 should see the same.
	if err := expect(c2Updates, &discovery.AdId{}, find, discovery.Attributes{wire.DiscoveryAttrPeer: peer}); err != nil {
		t.Error(err)
	}

	sg1Attrs := discovery.Attributes{
		wire.DiscoveryAttrDatabaseName:      "d",
		wire.DiscoveryAttrDatabaseBlessing:  "root:o:app1",
		wire.DiscoveryAttrSyncgroupName:     "sg1",
		wire.DiscoveryAttrSyncgroupBlessing: "root:o:app1:client1",
	}
	sg2Attrs := discovery.Attributes{
		wire.DiscoveryAttrDatabaseName:      "d",
		wire.DiscoveryAttrDatabaseBlessing:  "root:o:app1",
		wire.DiscoveryAttrSyncgroupName:     "sg2",
		wire.DiscoveryAttrSyncgroupBlessing: "root:o:app1:client1",
	}

	// Then we should see an update for the created syncgroup.
	var sg1AdId discovery.AdId
	if err := expect(c1Updates, &sg1AdId, find, sg1Attrs); err != nil {
		t.Error(err)
	}

	// Now update the spec to add client2 to the permissions.
	spec.Perms = tu.DefaultPerms(wire.AllSyncgroupTags, "root:server", "root:o:app1:client1", "root:o:app1:client2")
	if err := d.SyncgroupForId(sgId).SetSpec(ctx, spec, ""); err != nil {
		t.Fatalf("sg.SetSpec failed: %v", err)
	}

	// Client1 should see a lost and a found message.
	if err := expect(c1Updates, &sg1AdId, both, sg1Attrs); err != nil {
		t.Error(err)
	}
	// Client2 should just now see the found message.
	if err := expect(c2Updates, &sg1AdId, find, sg1Attrs); err != nil {
		t.Error(err)
	}

	// Now create a second syncgroup.
	sg2Id := d.Syncgroup(ctx, "sg2").Id()
	spec2 := wire.SyncgroupSpec{
		Description: "test syncgroup sg2",
		Perms:       tu.DefaultPerms(wire.AllSyncgroupTags, "root:server", "root:o:app1:client1", "root:o:app1:client2"),
		Collections: []wire.Id{collection2.Id()},
	}
	createSyncgroup(t, ctx, d, sg2Id, spec2, verror.ID(""))

	// Both clients should see the new syncgroup.
	var sg2AdId discovery.AdId
	if err := expect(c1Updates, &sg2AdId, find, sg2Attrs); err != nil {
		t.Error(err)
	}
	if err := expect(c2Updates, &sg2AdId, find, sg2Attrs); err != nil {
		t.Error(err)
	}

	spec2.Perms = tu.DefaultPerms(wire.AllSyncgroupTags, "root:server", "root:o:app1:client1")
	if err := d.SyncgroupForId(sg2Id).SetSpec(ctx, spec2, ""); err != nil {
		t.Fatalf("sg.SetSpec failed: %v", err)
	}
	if err := expect(c2Updates, &sg2AdId, lose, sg2Attrs); err != nil {
		t.Error(err)
	}
}

func scanAs(ctx *context.T, rootp security.Principal, as string) (<-chan discovery.Update, error) {
	idp := testutil.IDProviderFromPrincipal(rootp)
	p := testutil.NewPrincipal()
	if err := idp.Bless(p, as); err != nil {
		return nil, err
	}
	ctx, err := v23.WithPrincipal(ctx, p)
	if err != nil {
		return nil, err
	}
	dis, err := syncdis.NewDiscovery(ctx, "", 0)
	if err != nil {
		return nil, err
	}
	return dis.Scan(ctx, `v.InterfaceName="v.io/x/ref/services/syncbase/server/interfaces/Sync"`)
}

const (
	lose = "lose"
	find = "find"
	both = "both"
)

func expect(ch <-chan discovery.Update, id *discovery.AdId, typ string, want discovery.Attributes) error {
	select {
	case u := <-ch:
		if (u.IsLost() && typ == find) || (!u.IsLost() && typ == lose) {
			return fmt.Errorf("IsLost mismatch.  Got %v, wanted %v", u, typ)
		}
		ad := u.Advertisement()
		got := ad.Attributes
		if id.IsValid() {
			if *id != ad.Id {
				return fmt.Errorf("mismatched id, got %v, want %v", ad.Id, id)
			}
		} else {
			*id = ad.Id
		}
		if !reflect.DeepEqual(got, want) {
			return fmt.Errorf("got %v, want %v", got, want)
		}
		if typ == both {
			typ = lose
			if u.IsLost() {
				typ = find
			}
			return expect(ch, id, typ, want)
		}
		return nil
	case <-time.After(2 * time.Second):
		return fmt.Errorf("timed out")
	}
}

func TestGlobalSyncbaseDiscovery(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()
	// Create syncbase discovery service with mock neighborhood discovery.
	p := mock.New()
	df, _ := idiscovery.NewFactory(ctx, p)
	defer df.Shutdown()
	fdiscovery.InjectFactory(df)
	const globalDiscoveryPath = "syncdis"
	gdis, err := global.New(ctx, globalDiscoveryPath)
	if err != nil {
		t.Fatal(err)
	}
	dis, err := syncdis.NewDiscovery(ctx, globalDiscoveryPath, 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	// Start the syncbase discovery scan.
	scanCh, err := dis.Scan(ctx, ``)
	if err != nil {
		t.Fatal(err)
	}
	// The ids of all the ads match.
	ad := &discovery.Advertisement{
		Id:            discovery.AdId{1, 2, 3},
		InterfaceName: "v.io/a",
		Addresses:     []string{"/h1:123/x", "/h2:123/y"},
		Attributes:    discovery.Attributes{"a": "v"},
	}
	newad := &discovery.Advertisement{
		Id:            discovery.AdId{1, 2, 3},
		InterfaceName: "v.io/a",
		Addresses:     []string{"/h1:123/x", "/h3:123/z"},
		Attributes:    discovery.Attributes{"a": "v"},
	}
	newadattach := &discovery.Advertisement{
		Id:            discovery.AdId{1, 2, 3},
		InterfaceName: "v.io/a",
		Addresses:     []string{"/h1:123/x", "/h3:123/z"},
		Attributes:    discovery.Attributes{"a": "v"},
		Attachments:   discovery.Attachments{"k": []byte("v")},
	}
	adinfo := &idiscovery.AdInfo{
		Ad:          *ad,
		Hash:        idiscovery.AdHash{1, 2, 3},
		TimestampNs: time.Now().UnixNano(),
	}
	newadattachinfo := &idiscovery.AdInfo{
		Ad:          *newadattach,
		Hash:        idiscovery.AdHash{3, 2, 1},
		TimestampNs: time.Now().UnixNano(),
	}
	// Advertise ad via both services, we should get a found update.
	adCtx, adCancel := context.WithCancel(ctx)
	p.RegisterAd(adinfo)
	adStopped, err := gdis.Advertise(adCtx, ad, nil)
	if err != nil {
		t.Error(err)
	}
	if err := expect(scanCh, &ad.Id, find, ad.Attributes); err != nil {
		t.Error(err)
	}
	// Lost via both services, we should get a lost update.
	p.UnregisterAd(adinfo)
	adCancel()
	<-adStopped
	if err := expect(scanCh, &ad.Id, lose, ad.Attributes); err != nil {
		t.Error(err)
	}
	// Advertise via mock plugin, we should get a found update.
	p.RegisterAd(adinfo)
	if err := expect(scanCh, &ad.Id, find, ad.Attributes); err != nil {
		t.Error(err)
	}
	// Advertise via global discovery with a newer changed ad, we should get a lost ad
	// update and found ad update.
	adCtx, adCancel = context.WithCancel(ctx)
	adStopped, err = gdis.Advertise(adCtx, newad, nil)
	if err != nil {
		t.Error(err)
	}
	if err := expect(scanCh, &newad.Id, both, newad.Attributes); err != nil {
		t.Error(err)
	}
	// If we advertise an ad from neighborhood discovery that is identical to the
	// ad found from global discovery, but has attachments, we should prefer it.
	p.RegisterAd(newadattachinfo)
	if err := expect(scanCh, &newad.Id, both, newad.Attributes); err != nil {
		t.Error(err)
	}
	adCancel()
	<-adStopped
}

func TestListenForInvites(t *testing.T) {
	_, ctx, sName, rootp, cleanup := tu.SetupOrDieCustom(
		"o:app1:client1",
		"server",
		tu.DefaultPerms(access.AllTypicalTags(), "root:o:app1:client1"))
	defer cleanup()
	service := syncbase.NewService(sName)
	d1 := tu.CreateDatabase(t, ctx, service, "d1")
	d2 := tu.CreateDatabase(t, ctx, service, "d2")

	collections := []wire.Id{}
	for _, d := range []syncbase.Database{d1, d2} {
		for _, n := range []string{"c1", "c2"} {
			c := tu.CreateCollection(t, ctx, d, d.Id().Name+n)
			collections = append(collections, c.Id())
		}
	}

	d1invites, err := listenForInvitesAs(ctx, rootp, "o:app1:client1", d1.Id())
	if err != nil {
		panic(err)
	}
	d2invites, err := listenForInvitesAs(ctx, rootp, "o:app1:client1", d2.Id())
	if err != nil {
		panic(err)
	}

	if err := advertiseAndFindSyncgroup(t, ctx, d1, collections[0], d1invites); err != nil {
		t.Error(err)
	}
	if err := advertiseAndFindSyncgroup(t, ctx, d2, collections[2], d2invites); err != nil {
		t.Error(err)
	}
	if err := advertiseAndFindSyncgroup(t, ctx, d1, collections[1], d1invites); err != nil {
		t.Error(err)
	}
	if err := advertiseAndFindSyncgroup(t, ctx, d2, collections[3], d2invites); err != nil {
		t.Error(err)
	}
}

func listenForInvitesAs(ctx *context.T, rootp security.Principal, as string, db wire.Id) (<-chan syncdis.Invite, error) {
	ch := make(chan syncdis.Invite)
	return ch, syncdis.ListenForInvites(tu.NewCtx(ctx, rootp, as), db, ch)
}

func advertiseAndFindSyncgroup(
	t *testing.T,
	ctx *context.T,
	d syncbase.Database,
	collection wire.Id,
	invites <-chan syncdis.Invite) error {
	sgId := wire.Id{Name: collection.Name + "sg", Blessing: collection.Blessing}
	ctx.Infof("creating %v", sgId)
	spec := wire.SyncgroupSpec{
		Description: "test syncgroup",
		Perms:       tu.DefaultPerms(wire.AllSyncgroupTags, "root:server", "root:o:app1:client1"),
		Collections: []wire.Id{collection},
	}
	createSyncgroup(t, ctx, d, sgId, spec, verror.ID(""))

	// We should see an invite for sg1.
	inv := <-invites
	if inv.Syncgroup != sgId {
		return fmt.Errorf("got %v, want %v", inv.Syncgroup, sgId)
	}
	return nil
}

func createSyncgroup(t *testing.T, ctx *context.T, d syncbase.Database, sgId wire.Id, spec wire.SyncgroupSpec, errID verror.ID) syncbase.Syncgroup {
	sg := d.SyncgroupForId(sgId)
	info := wire.SyncgroupMemberInfo{SyncPriority: 8}
	if err := sg.Create(ctx, spec, info); verror.ErrorID(err) != errID {
		tu.Fatalf(t, "Create SG %+v failed: %v", sgId, err)
	}
	return sg
}

func raisePeer(t *testing.T, label string, ctx *context.T) <-chan struct{} {
	ad := discovery.Advertisement{
		InterfaceName: interfaces.SyncDesc.PkgPath + "/" + interfaces.SyncDesc.Name,
		Attributes:    map[string]string{wire.DiscoveryAttrPeer: label},
	}
	suffix := ""
	eps := udiscovery.ToEndpoints("addr1:123")
	mockServer := udiscovery.NewMockServer(eps)
	done, err := idiscovery.AdvertiseServer(ctx, nil, mockServer, suffix, &ad, nil)
	if err != nil {
		t.Fatal(err)
	}
	return done
}

// Do basic tests to check that listen for peers functions as expected.
// 3 peers are raised in this test and multiple scanners are used.
// Note: All scanners will see the next found peer even if they haven't
// consumed the channel for it yet. The effective buffer size is chan size + 1.
func TestListenForPeers(t *testing.T) {
	// Setup a base context that is given a rooted principal.
	baseCtx, shutdown := test.V23Init()
	defer shutdown()

	rootp := testutil.NewPrincipal("root")
	ctx := tu.NewCtx(baseCtx, rootp, "o:app")

	// Discovery is also mocked out for future "fake" ads.
	df, _ := idiscovery.NewFactory(ctx, mock.New())
	fdiscovery.InjectFactory(df)

	// Raise Peer 1.
	ctx1, cancel1 := context.WithCancel(ctx)
	defer cancel1()
	done1 := raisePeer(t, "peer1", ctx1)

	// 1a: Peer 1 can only see themselves.
	ctx1a, cancel1a := context.WithCancel(ctx1)
	peerChan1a, err := listenForPeersAs(ctx1a, rootp, "client1")
	if err != nil {
		panic(err)
	}
	checkExpectedPeers(t, peerChan1a, 1, 0)
	cancel1a()
	confirmCleanupPeer(t, peerChan1a)

	// Raise Peer 2.
	ctx2, cancel2 := context.WithCancel(ctx)
	defer cancel2()
	raisePeer(t, "peer2", ctx2)

	// 2a is special and will run until the end of the test. It reads 0 peers now.
	ctx2a, cancel2a := context.WithCancel(ctx2)
	peerChan2a, err := listenForPeersAs(ctx2a, rootp, "client2")
	if err != nil {
		panic(err)
	}

	// 2e is special and will run until the end of the test. It reads 2 peers now.
	ctx2e, cancel2e := context.WithCancel(ctx2)
	peerChan2e, err := listenForPeersAs(ctx2e, rootp, "client2")
	if err != nil {
		panic(err)
	}
	checkExpectedPeers(t, peerChan2e, 2, 0)

	// 1b and 2b: Peer 1 and Peer 2 can see each other.
	ctx1b, cancel1b := context.WithCancel(ctx1)
	peerChan1b, err := listenForPeersAs(ctx1b, rootp, "client1")
	if err != nil {
		panic(err)
	}
	checkExpectedPeers(t, peerChan1b, 2, 0)
	cancel1b()
	confirmCleanupPeer(t, peerChan1b)

	ctx2b, cancel2b := context.WithCancel(ctx2)
	peerChan2b, err := listenForPeersAs(ctx2b, rootp, "client2")
	if err != nil {
		panic(err)
	}
	checkExpectedPeers(t, peerChan2b, 2, 0)
	cancel2b()
	confirmCleanupPeer(t, peerChan2b)

	// Raise Peer 3.
	ctx3, cancel3 := context.WithCancel(ctx)
	defer cancel3()
	done3 := raisePeer(t, "peer3", ctx3)

	// 2c is special and will run until the end of the test. It reads 3 peers now.
	ctx2c, cancel2c := context.WithCancel(ctx2)
	peerChan2c, err := listenForPeersAs(ctx2c, rootp, "client2")
	if err != nil {
		panic(err)
	}
	checkExpectedPeers(t, peerChan2c, 3, 0)

	// Stop Peer 1 by canceling its context and waiting for the ad to disappear.
	cancel1()
	<-done1

	// The "remove" notification goroutines might race, so wait a little bit.
	// It is okay to wait since this is purely for test simplification purposes.
	<-time.After(time.Millisecond * 10)

	// 2c also sees lost 1.
	checkExpectedPeers(t, peerChan2c, 0, 1)

	// 2d and 3d: Peer 2 and Peer 3 see each other and nobody else.
	ctx2d, cancel2d := context.WithCancel(ctx2)
	peerChan2d, err := listenForPeersAs(ctx2d, rootp, "client2")
	if err != nil {
		panic(err)
	}
	checkExpectedPeers(t, peerChan2d, 2, 0)
	cancel2d()
	confirmCleanupPeer(t, peerChan2d)

	ctx3d, cancel3d := context.WithCancel(ctx3)
	peerChan3d, err := listenForPeersAs(ctx3d, rootp, "client3")
	if err != nil {
		panic(err)
	}
	checkExpectedPeers(t, peerChan3d, 2, 0)
	cancel3d()
	confirmCleanupPeer(t, peerChan3d)

	// Stop Peer 3 by canceling its context and waiting for the ad to disappear.
	cancel3()
	<-done3

	// The "remove" notification goroutines might race, so wait a little bit.
	// It is okay to wait since this is purely for test simplification purposes.
	<-time.After(time.Millisecond * 10)

	// 2c also sees lost 3.
	checkExpectedPeers(t, peerChan2c, 0, 1)

	// We're done with 2c, who saw 3 updates (1, 2, 3) and 2 losses (1 and 3)
	cancel2c()
	confirmCleanupPeer(t, peerChan2c)

	// 2a should see 1, 2, and lose 1. It won't notice #3 since the channel won't buffer it.
	checkExpectedPeers(t, peerChan2a, 2, 1)
	cancel2a()
	confirmCleanupPeer(t, peerChan2a)

	// 2e has seen 2 found updates earlier, so peer 3 is buffered in the channel.
	// We should see peer 3 and two loss events.
	checkExpectedPeers(t, peerChan2e, 1, 2)
	cancel2e()
	confirmCleanupPeer(t, peerChan2e)
}

func listenForPeersAs(ctx *context.T, rootp security.Principal, as string) (<-chan syncdis.Peer, error) {
	ch := make(chan syncdis.Peer)
	return ch, syncdis.ListenForPeers(tu.NewCtx(ctx, rootp, as), ch)
}

func checkExpectedPeers(t *testing.T, ch <-chan syncdis.Peer, found int, lost int) {
	counter := 0
	for i := 0; i < found+lost; i++ {
		select {
		case peer, ok := <-ch:
			if !ok {
				t.Error("peer channel shouldn't be closed yet")
			} else {
				if !peer.Lost {
					counter++
				}
			}
		case <-time.After(time.Second * 1):
			t.Errorf("timed out without seeing enough peers. Found %d in %d updates, but needed %d in %d", counter, i, found, found+lost)
			return
		}
	}

	if counter != found {
		t.Errorf("Found %d peers, expected %d", counter, found)
	}
}

func confirmCleanupPeer(t *testing.T, ch <-chan syncdis.Peer) {
	unwanted, ok := <-ch
	if ok {
		t.Errorf("found unwanted peer update %v", unwanted)
	}
}

// Since the basic found/loss is tested by listenForPeers, this test will focus
// on whether the AdvertiseApp and ListenForPeers functions together.
func TestAdvertiseAndListenForAppPeers(t *testing.T) {
	// Setup a base context that is given a rooted principal.
	baseCtx, shutdown := test.V23Init()
	defer shutdown()

	rootp := testutil.NewPrincipal("dev.v.io")
	ctx := tu.NewCtx(baseCtx, rootp, "o:app")

	// Discovery is also mocked out to mock ads and scans.
	df, _ := idiscovery.NewFactory(ctx, mock.New())
	fdiscovery.InjectFactory(df)

	// First, start Eve, who listens on "todos", "croupier", and syncslides.
	ctxMain, cancelMain := context.WithCancel(baseCtx)
	todosChan, err := listenForAppPeersAs(ctxMain, rootp, "o:todos:eve")
	if err != nil {
		panic(err)
	}
	croupierChan, err := listenForAppPeersAs(ctxMain, rootp, "o:croupier:eve")
	if err != nil {
		panic(err)
	}
	syncslidesChan, err := listenForAppPeersAs(ctxMain, rootp, "o:syncslides:eve")
	if err != nil {
		panic(err)
	}

	// Create other application peers.
	ctxTodosA := tu.NewCtx(baseCtx, rootp, "o:todos:alice")
	ctxTodosB := tu.NewCtx(baseCtx, rootp, "o:todos:bob")
	ctxTodosC := tu.NewCtx(baseCtx, rootp, "o:todos:carol")
	ctxCroupierA := tu.NewCtx(baseCtx, rootp, "o:croupier:alice")

	// Advertise "todos" Peer Alice.
	ctxTodosAAd, cancelTodosAAd := context.WithCancel(ctxTodosA)
	todosDoneAAd, err := syncdis.AdvertiseApp(ctxTodosAAd, nil)
	if err != nil {
		panic(err)
	}

	// Advertise "todos" Peer Bob.
	ctxTodosBAd, cancelTodosBAd := context.WithCancel(ctxTodosB)
	todosDoneBAd, err := syncdis.AdvertiseApp(ctxTodosBAd, nil)
	if err != nil {
		panic(err)
	}

	// Start short-term listeners. The one on "todos" sees 2 peers. The one for
	// "croupier" sees 0.
	ctxT1, cancelT1 := context.WithCancel(ctx)
	todosChan1, err := listenForAppPeersAs(ctxT1, rootp, "o:todos:eve")
	if err != nil {
		panic(err)
	}
	checkExpectedAppPeers(t, todosChan1, []string{"dev.v.io:o:todos:alice", "dev.v.io:o:todos:bob"}, nil)
	cancelT1()
	confirmCleanupAppPeer(t, todosChan1)

	ctxT2, cancelT2 := context.WithCancel(ctx)
	croupierChan1, err := listenForAppPeersAs(ctxT2, rootp, "o:croupier:eve")
	if err != nil {
		panic(err)
	}
	checkExpectedAppPeers(t, croupierChan1, nil, nil)
	cancelT2()
	confirmCleanupAppPeer(t, croupierChan1)

	// Let's also consume 2 with Eve's todos scanner.
	checkExpectedAppPeers(t, todosChan, []string{"dev.v.io:o:todos:alice", "dev.v.io:o:todos:bob"}, nil)

	// Stop advertising todos Peer A.
	cancelTodosAAd()
	<-todosDoneAAd

	// The "remove" notification goroutines might race, so wait a little bit.
	// It is okay to wait since this is purely for test simplification purposes.
	<-time.After(time.Millisecond * 10)

	// So Eve's todos scanner will now see a loss.
	checkExpectedAppPeers(t, todosChan, nil, []string{"dev.v.io:o:todos:alice"})

	// Start advertising todos Carol and croupier Alice.
	ctxTodosCAd, cancelTodosCAd := context.WithCancel(ctxTodosC)
	todosDoneCAd, err := syncdis.AdvertiseApp(ctxTodosCAd, nil)
	if err != nil {
		panic(err)
	}
	ctxCroupierAAd, cancelCroupierAAd := context.WithCancel(ctxCroupierA)
	croupierDoneAAd, err := syncdis.AdvertiseApp(ctxCroupierAAd, nil)
	if err != nil {
		panic(err)
	}

	// Expect Eve's todos and croupier to both see 1 new peer each.
	checkExpectedAppPeers(t, todosChan, []string{"dev.v.io:o:todos:carol"}, nil)
	checkExpectedAppPeers(t, croupierChan, []string{"dev.v.io:o:croupier:alice"}, nil)

	// Stop advertising todos Peer B and Peer C and croupier Peer A.
	cancelTodosBAd()
	cancelTodosCAd()
	cancelCroupierAAd()
	<-todosDoneBAd
	<-todosDoneCAd
	<-croupierDoneAAd

	// The "remove" notification goroutines might race, so wait a little bit.
	// It is okay to wait since this is purely for test simplification purposes.
	<-time.After(time.Millisecond * 10)

	// Expect that Eve's todos channel will see 2 losses. Croupier's loses 1.
	checkExpectedAppPeers(t, todosChan, nil, []string{"dev.v.io:o:todos:bob", "dev.v.io:o:todos:carol"})
	checkExpectedAppPeers(t, croupierChan, nil, []string{"dev.v.io:o:croupier:alice"})

	// Close the scanners. No additional found/lost events should be detected by Eve.
	cancelMain()
	confirmCleanupAppPeer(t, todosChan)
	confirmCleanupAppPeer(t, croupierChan)
	confirmCleanupAppPeer(t, syncslidesChan)
}

func listenForAppPeersAs(ctx *context.T, rootp security.Principal, as string) (<-chan syncdis.AppPeer, error) {
	ch := make(chan syncdis.AppPeer)
	return ch, syncdis.ListenForAppPeers(tu.NewCtx(ctx, rootp, as), ch)
}

func checkExpectedAppPeers(t *testing.T, ch <-chan syncdis.AppPeer, foundList []string, lostList []string) {
	lost := len(lostList)
	found := len(foundList)
	counter := 0
	for i := 0; i < found+lost; i++ {
		select {
		case peer, ok := <-ch:
			if !ok {
				t.Error("app peer channel shouldn't be closed yet")
			} else {
				// Verify that found peers are in the foundList and that lost peers are in the lostList.
				if !peer.Lost {
					counter++
					inFound := false
					for _, blessings := range foundList {
						if peer.Blessings == blessings {
							inFound = true
							break
						}
					}
					if !inFound {
						t.Errorf("Expected found blessings in %v, but got the app peer: %v", lostList, peer)
					}
				} else {
					inLost := false
					for _, blessings := range lostList {
						if peer.Blessings == blessings {
							inLost = true
							break
						}
					}
					if !inLost {
						t.Errorf("Expected lost blessings in %v, but got the app peer: %v", lostList, peer)
					}

				}
			}
		case <-time.After(time.Second * 1):
			t.Errorf("timed out without seeing enough app peers. Found %d in %d updates, but needed %d in %d", counter, i, found, found+lost)
			return
		}
	}

	// Having gone through found + lost peers, "counter" must match the number we expected to find.
	if counter != found {
		t.Errorf("Found %d app peers, expected %d", counter, found)
	}
}

func confirmCleanupAppPeer(t *testing.T, ch <-chan syncdis.AppPeer) {
	unwanted, ok := <-ch
	if ok {
		t.Errorf("found unwanted app peer update %v", unwanted)
	}
}

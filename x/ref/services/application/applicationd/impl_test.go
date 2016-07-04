// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security"
	"v.io/v23/services/application"
	"v.io/v23/verror"
	appd "v.io/x/ref/services/application/applicationd"
	"v.io/x/ref/services/repository"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

func newPublisherSignature(t *testing.T, ctx *context.T, msg []byte) (security.Blessings, security.Signature) {
	// Generate publisher blessings
	p := v23.GetPrincipal(ctx)
	b, err := p.BlessSelf("publisher")
	if err != nil {
		t.Fatal(err)
	}
	sig, err := p.Sign(msg)
	if err != nil {
		t.Fatal(err)
	}
	return b, sig
}

func checkEnvelope(t *testing.T, ctx *context.T, expected application.Envelope, stub repository.ApplicationClientStub, profiles ...string) {
	if output, err := stub.Match(ctx, profiles); err != nil {
		t.Fatal(testutil.FormatLogLine(2, "Match() failed: %v", err))
	} else if !reflect.DeepEqual(expected, output) {
		t.Fatal(testutil.FormatLogLine(2, "Incorrect Match output: expected %#v, got %#v", expected, output))
	}
}

func checkNoEnvelope(t *testing.T, ctx *context.T, stub repository.ApplicationClientStub, profiles ...string) {
	if _, err := stub.Match(ctx, profiles); err == nil || verror.ErrorID(err) != verror.ErrNoExist.ID {
		t.Fatal(testutil.FormatLogLine(2, "Unexpected error: expected %v, got %v", verror.ErrNoExist, err))
	}
}

func checkProfiles(t *testing.T, ctx *context.T, stub repository.ApplicationClientStub, expected ...string) {
	if output, err := stub.Profiles(ctx); err != nil {
		t.Fatal(testutil.FormatLogLine(2, "Profiles() failed: %v", err))
	} else if !reflect.DeepEqual(expected, output) {
		t.Fatal(testutil.FormatLogLine(2, "Incorrect Profiles output: expected %v, got %v", expected, output))
	}
}

func checkNoProfile(t *testing.T, ctx *context.T, stub repository.ApplicationClientStub) {
	if _, err := stub.Profiles(ctx); err == nil || verror.ErrorID(err) != verror.ErrNoExist.ID {
		t.Fatal(testutil.FormatLogLine(2, "Unexpected error: expected %v, got %v", verror.ErrNoExist, err))
	}
}

// TestInterface tests that the implementation correctly implements
// the Application interface.
func TestInterface(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	dir, prefix := "", ""
	store, err := ioutil.TempDir(dir, prefix)
	if err != nil {
		t.Fatalf("TempDir(%q, %q) failed: %v", dir, prefix, err)
	}
	defer os.RemoveAll(store)
	dispatcher, err := appd.NewDispatcher(store)
	if err != nil {
		t.Fatalf("NewDispatcher() failed: %v", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	_, server, err := v23.WithNewDispatchingServer(ctx, "", dispatcher)
	if err != nil {
		t.Fatalf("NewServer(%v) failed: %v", dispatcher, err)
	}
	endpoint := server.Status().Endpoints[0].String()

	// Create client stubs for talking to the server.
	stub := repository.ApplicationClient(naming.JoinAddressName(endpoint, "search"))
	stubV0 := repository.ApplicationClient(naming.JoinAddressName(endpoint, "search/v0"))
	stubV1 := repository.ApplicationClient(naming.JoinAddressName(endpoint, "search/v1"))
	stubV2 := repository.ApplicationClient(naming.JoinAddressName(endpoint, "search/v2"))
	stubV3 := repository.ApplicationClient(naming.JoinAddressName(endpoint, "search/v3"))

	blessings, sig := newPublisherSignature(t, ctx, []byte("binarycontents"))

	// Create example envelopes.
	envelopeV1 := application.Envelope{
		Args: []string{"--help"},
		Env:  []string{"DEBUG=1"},
		Binary: application.SignedFile{
			File:      "/v23/name/of/binary",
			Signature: sig,
		},
		Publisher: blessings,
	}
	envelopeV2 := application.Envelope{
		Args: []string{"--verbose"},
		Env:  []string{"DEBUG=0"},
		Binary: application.SignedFile{
			File:      "/v23/name/of/binary",
			Signature: sig,
		},
		Publisher: blessings,
	}
	envelopeV3 := application.Envelope{
		Args: []string{"--verbose", "--spiffynewflag"},
		Env:  []string{"DEBUG=0"},
		Binary: application.SignedFile{
			File:      "/v23/name/of/binary",
			Signature: sig,
		},
		Publisher: blessings,
	}
	checkNoProfile(t, ctx, stub)

	// Test Put(), adding a number of application envelopes.
	if err := stubV1.Put(ctx, "base", envelopeV1, false); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}
	if err := stubV1.Put(ctx, "media", envelopeV1, false); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}
	if err := stubV2.Put(ctx, "base", envelopeV2, false); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}
	if err := stub.Put(ctx, "base", envelopeV1, false); err == nil || verror.ErrorID(err) != appd.ErrInvalidSuffix.ID {
		t.Fatalf("Unexpected error: expected %v, got %v", appd.ErrInvalidSuffix, err)
	}

	// Test Match() against versioned names, trying profiles that do and
	// don't have any envelopes uploaded.
	checkNoEnvelope(t, ctx, stubV2)
	checkEnvelope(t, ctx, envelopeV2, stubV2, "base")
	checkNoEnvelope(t, ctx, stubV2, "media")
	checkEnvelope(t, ctx, envelopeV2, stubV2, "base", "media")
	checkEnvelope(t, ctx, envelopeV2, stubV2, "media", "base")

	// Test that Match() against a name without a version suffix returns the
	// latest.
	checkEnvelope(t, ctx, envelopeV2, stub, "base", "media")
	checkEnvelope(t, ctx, envelopeV1, stub, "media")

	checkProfiles(t, ctx, stub, "base", "media")
	checkProfiles(t, ctx, stubV1, "base", "media")
	checkProfiles(t, ctx, stubV2, "base")
	checkNoProfile(t, ctx, stubV3)

	// Test that if we add another envelope for a version that's the highest
	// in sort order, the new envelope becomes the latest.
	if err := stubV3.Put(ctx, "base", envelopeV3, false); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}
	checkEnvelope(t, ctx, envelopeV3, stub, "base", "media")
	checkProfiles(t, ctx, stubV3, "base")

	// Test that this is not based on time but on sort order.
	envelopeV0 := application.Envelope{
		Args: []string{"--help", "--zeroth"},
		Env:  []string{"DEBUG=1"},
		Binary: application.SignedFile{
			File:      "/v23/name/of/binary",
			Signature: sig,
		},
		Publisher: blessings,
	}
	if err := stubV0.Put(ctx, "base", envelopeV0, false); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}
	checkEnvelope(t, ctx, envelopeV3, stub, "base", "media")

	// Test Glob
	matches, _, err := testutil.GlobName(ctx, naming.JoinAddressName(endpoint, ""), "...")
	if err != nil {
		t.Errorf("Unexpected Glob error: %v", err)
	}
	expected := []string{
		"",
		"search",
		"search/v0",
		"search/v1",
		"search/v2",
		"search/v3",
	}
	if !reflect.DeepEqual(matches, expected) {
		t.Errorf("unexpected Glob results. Got %q, want %q", matches, expected)
	}

	// Put cannot replace the envelope for v0-base when overwrite is false.
	if err := stubV0.Put(ctx, "base", envelopeV2, false); err == nil || verror.ErrorID(err) != verror.ErrExist.ID {
		t.Fatalf("Unexpected error: expected %v, got %v", appd.ErrInvalidSuffix, err)
	}
	checkEnvelope(t, ctx, envelopeV0, stubV0, "base")
	// Put can replace the envelope for v0-base when overwrite is true.
	if err := stubV0.Put(ctx, "base", envelopeV2, true); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}
	checkEnvelope(t, ctx, envelopeV2, stubV0, "base")

	// Test Remove(), trying to remove both existing and non-existing
	// application envelopes.
	if err := stubV1.Remove(ctx, "base"); err != nil {
		t.Fatalf("Remove() failed: %v", err)
	}
	checkNoEnvelope(t, ctx, stubV1)
	checkEnvelope(t, ctx, envelopeV1, stubV1, "media")
	checkNoEnvelope(t, ctx, stubV1, "base")
	checkEnvelope(t, ctx, envelopeV1, stubV1, "base", "media")
	checkEnvelope(t, ctx, envelopeV1, stubV1, "media", "base")

	if err := stubV1.Remove(ctx, "base"); err == nil || verror.ErrorID(err) != verror.ErrNoExist.ID {
		t.Fatalf("Unexpected error: expected %v, got %v", verror.ErrNoExist, err)
	}
	if err := stub.Remove(ctx, "base"); err != nil {
		t.Fatalf("Remove() failed: %v", err)
	}
	checkNoProfile(t, ctx, stubV0)
	checkProfiles(t, ctx, stubV1, "media")
	checkNoProfile(t, ctx, stubV2)
	checkNoProfile(t, ctx, stubV3)
	checkProfiles(t, ctx, stub, "media")
	if err := stubV2.Remove(ctx, "media"); err == nil || verror.ErrorID(err) != verror.ErrNoExist.ID {
		t.Fatalf("Unexpected error: expected %v, got %v", verror.ErrNoExist, err)
	}
	if err := stubV1.Remove(ctx, "media"); err != nil {
		t.Fatalf("Remove() failed: %v", err)
	}
	checkNoProfile(t, ctx, stub)

	// Finally, use Match() to test that Remove really removed the
	// application envelopes.
	checkNoEnvelope(t, ctx, stubV1, "base")
	checkNoEnvelope(t, ctx, stubV1, "media")
	checkNoEnvelope(t, ctx, stubV2, "base")

	if err := stubV0.Put(ctx, "base", envelopeV0, false); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}
	if err := stubV1.Put(ctx, "base", envelopeV1, false); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}
	if err := stubV1.Put(ctx, "media", envelopeV1, false); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}
	if err := stubV2.Put(ctx, "base", envelopeV2, false); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}
	if err := stubV3.Put(ctx, "base", envelopeV3, false); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}
	if err := stub.Remove(ctx, "*"); err != nil {
		t.Fatalf("Remove() failed: %v", err)
	}
	if err := stub.Remove(ctx, "*"); err == nil || verror.ErrorID(err) != verror.ErrNoExist.ID {
		t.Fatalf("Unexpected error: expected %v, got %v", verror.ErrNoExist, err)
	}
	checkNoProfile(t, ctx, stub)

	// Shutdown the application repository server.
	cancel()
	<-server.Closed()
}

func TestPreserveAcrossRestarts(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	dir, prefix := "", ""
	storedir, err := ioutil.TempDir(dir, prefix)
	if err != nil {
		t.Fatalf("TempDir(%q, %q) failed: %v", dir, prefix, err)
	}
	defer os.RemoveAll(storedir)

	dispatcher, err := appd.NewDispatcher(storedir)
	if err != nil {
		t.Fatalf("NewDispatcher() failed: %v", err)
	}

	sctx, cancel := context.WithCancel(ctx)
	_, server, err := v23.WithNewDispatchingServer(sctx, "", dispatcher)
	if err != nil {
		t.Fatalf("Serve(%v) failed: %v", dispatcher, err)
	}
	endpoint := server.Status().Endpoints[0].String()

	// Create client stubs for talking to the server.
	stubV1 := repository.ApplicationClient(naming.JoinAddressName(endpoint, "search/v1"))

	blessings, sig := newPublisherSignature(t, ctx, []byte("binarycontents"))

	// Create example envelopes.
	envelopeV1 := application.Envelope{
		Args: []string{"--help"},
		Env:  []string{"DEBUG=1"},
		Binary: application.SignedFile{
			File:      "/v23/name/of/binary",
			Signature: sig,
		},
		Publisher: blessings,
	}

	if err := stubV1.Put(ctx, "media", envelopeV1, false); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}

	// There is content here now.
	checkEnvelope(t, ctx, envelopeV1, stubV1, "media")

	cancel()
	<-server.Closed()

	// Setup and start a second application server.
	dispatcher, err = appd.NewDispatcher(storedir)
	if err != nil {
		t.Fatalf("NewDispatcher() failed: %v", err)
	}

	_, server, err = v23.WithNewDispatchingServer(ctx, "", dispatcher)
	if err != nil {
		t.Fatalf("NewServer(%v) failed: %v", dispatcher, err)
	}
	endpoint = server.Status().Endpoints[0].String()

	stubV1 = repository.ApplicationClient(naming.JoinAddressName(endpoint, "search/v1"))

	checkEnvelope(t, ctx, envelopeV1, stubV1, "media")
}

// TestTidyNow tests that TidyNow operates correctly.
func TestTidyNow(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	dir, prefix := "", ""
	store, err := ioutil.TempDir(dir, prefix)
	if err != nil {
		t.Fatalf("TempDir(%q, %q) failed: %v", dir, prefix, err)
	}
	defer os.RemoveAll(store)
	dispatcher, err := appd.NewDispatcher(store)
	if err != nil {
		t.Fatalf("NewDispatcher() failed: %v", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	_, server, err := v23.WithNewDispatchingServer(ctx, "", dispatcher)
	if err != nil {
		t.Fatalf("NewServer(%v) failed: %v", dispatcher, err)
	}
	defer func() {
		cancel()
		<-server.Closed()
	}()
	endpoint := server.Status().Endpoints[0].String()

	// Create client stubs for talking to the server.
	stub := repository.ApplicationClient(naming.JoinAddressName(endpoint, "search"))
	stubs := make([]repository.ApplicationClientStub, 0)
	for _, vn := range []string{"v0", "v1", "v2", "v3"} {
		s := repository.ApplicationClient(naming.JoinAddressName(endpoint, fmt.Sprintf("search/%s", vn)))
		stubs = append(stubs, s)
	}
	blessings, sig := newPublisherSignature(t, ctx, []byte("binarycontents"))

	// Create example envelopes.
	envelopeV1 := application.Envelope{
		Args: []string{"--help"},
		Env:  []string{"DEBUG=1"},
		Binary: application.SignedFile{
			File:      "/v23/name/of/binary",
			Signature: sig,
		},
		Publisher: blessings,
	}
	envelopeV2 := application.Envelope{
		Args: []string{"--verbose"},
		Env:  []string{"DEBUG=0"},
		Binary: application.SignedFile{
			File:      "/v23/name/of/binary",
			Signature: sig,
		},
		Publisher: blessings,
	}
	envelopeV3 := application.Envelope{
		Args: []string{"--verbose", "--spiffynewflag"},
		Env:  []string{"DEBUG=0"},
		Binary: application.SignedFile{
			File:      "/v23/name/of/binary",
			Signature: sig,
		},
		Publisher: blessings,
	}

	stuffEnvelopes(t, ctx, stubs, []profEnvTuple{
		{
			&envelopeV1,
			[]string{"base", "media"},
		},
	})

	// Verify that we have one
	testGlob(t, ctx, endpoint, []string{
		"",
		"search",
		"search/v0",
	})

	// Tidy when already tidy does not alter.
	if err := stubs[0].TidyNow(ctx); err != nil {
		t.Errorf("TidyNow failed: %v", err)
	}
	testGlob(t, ctx, endpoint, []string{
		"",
		"search",
		"search/v0",
	})

	stuffEnvelopes(t, ctx, stubs, []profEnvTuple{
		{
			&envelopeV1,
			[]string{"base", "media"},
		},
		{
			&envelopeV2,
			[]string{"media"},
		},
		{
			&envelopeV3,
			[]string{"base"},
		},
	})

	// Now there are three envelopes which is one more than the
	// numberOfVersionsToKeep set for the test. However
	// we need both envelopes v0 and v2 to keep two versions for
	// profile media and envelopes v0 and v3 to keep two versions
	// for profile base so we continue to have three versions.
	if err := stubs[0].TidyNow(ctx); err != nil {
		t.Errorf("TidyNow failed: %v", err)
	}
	testGlob(t, ctx, endpoint, []string{
		"",
		"search",
		"search/v0",
		"search/v1",
		"search/v2",
	})

	// And the newest version for each profile differs because
	// not every version supports all profiles.
	checkEnvelope(t, ctx, envelopeV2, stub, "media")
	checkEnvelope(t, ctx, envelopeV3, stub, "base")

	// Test that we can add an envelope for v3 with profile media and after calling
	// TidyNow(), there will be all versions still in glob but v0 will only match profile
	// base and not have an envelope for profile media.
	if err := stubs[3].Put(ctx, "media", envelopeV3, false); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}

	if err := stubs[0].TidyNow(ctx); err != nil {
		t.Errorf("TidyNow failed: %v", err)
	}
	testGlob(t, ctx, endpoint, []string{
		"",
		"search",
		"search/v0",
		"search/v1",
		"search/v2",
		"search/v3",
	})

	checkEnvelope(t, ctx, envelopeV1, stubs[0], "base")
	checkNoEnvelope(t, ctx, stubs[0], "media")

	stuffEnvelopes(t, ctx, stubs, []profEnvTuple{
		{
			&envelopeV1,
			[]string{"base", "media"},
		},
		{
			&envelopeV2,
			[]string{"base", "media"},
		},
		{
			&envelopeV3,
			[]string{"base", "media"},
		},
		{
			&envelopeV3,
			[]string{"base", "media"},
		},
	})

	// Now there are four versions for all profiles so tidying
	// will remove the older versions.
	if err := stubs[0].TidyNow(ctx); err != nil {
		t.Errorf("TidyNow failed: %v", err)
	}

	testGlob(t, ctx, endpoint, []string{
		"",
		"search",
		"search/v2",
		"search/v3",
	})
}

type profEnvTuple struct {
	e *application.Envelope
	p []string
}

func testGlob(t *testing.T, ctx *context.T, endpoint string, expected []string) {
	matches, _, err := testutil.GlobName(ctx, naming.JoinAddressName(endpoint, ""), "...")
	if err != nil {
		t.Errorf("Unexpected Glob error: %v", err)
	}
	if !reflect.DeepEqual(matches, expected) {
		t.Errorf("unexpected Glob results. Got %q, want %q", matches, expected)
	}
}

func stuffEnvelopes(t *testing.T, ctx *context.T, stubs []repository.ApplicationClientStub, pets []profEnvTuple) {
	for i, pet := range pets {
		for _, profile := range pet.p {
			if err := stubs[i].Put(ctx, profile, *pet.e, true); err != nil {
				t.Fatalf("%d: Put(%v) failed: %v", i, pet, err)
			}
		}
	}
}

// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal_test

import (
	"testing"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/x/ref/examples/fortune"
	"v.io/x/ref/examples/fortune/fortuned/internal"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
)

func TestGet(t *testing.T) {
	ctx, client, shutdown := setup(t)
	defer shutdown()

	value, err := client.Get(ctx)
	if err != nil {
		t.Errorf("Should not error")
	}

	if value == "" {
		t.Errorf("Expected non-empty fortune")
	}
}

func TestAdd(t *testing.T) {
	ctx, client, shutdown := setup(t)
	defer shutdown()

	value := "Lucky numbers: 12 2 35 46 56 4"
	err := client.Add(ctx, value)
	if err != nil {
		t.Errorf("Should not error")
	}

	added, err := client.Has(ctx, value)
	if err != nil {
		t.Errorf("Should not error")
	}

	if !added {
		t.Errorf("Expected service to add fortune")
	}
}

func setup(t *testing.T) (*context.T, fortune.FortuneClientStub, v23.Shutdown) {
	ctx, shutdown := test.V23Init()

	authorizer := security.DefaultAuthorizer()
	impl := internal.NewImpl()
	service := fortune.FortuneServer(impl)
	name := ""

	_, server, err := v23.WithNewServer(ctx, name, service, authorizer)
	if err != nil {
		t.Errorf("Failure creating server: %v", err)
	}

	endpoint := server.Status().Endpoints[0].Name()
	client := fortune.FortuneClient(endpoint)

	return ctx, client, shutdown
}

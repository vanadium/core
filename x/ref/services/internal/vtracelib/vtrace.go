// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vtracelib

import (
	"v.io/v23/context"
	"v.io/v23/rpc"
	s_vtrace "v.io/v23/services/vtrace"
	"v.io/v23/uniqueid"
	"v.io/v23/verror"
	"v.io/v23/vtrace"
)

type vtraceService struct{}

func (v *vtraceService) Trace(ctx *context.T, _ rpc.ServerCall, id uniqueid.Id) (vtrace.TraceRecord, error) {
	store := vtrace.GetStore(ctx)
	tr := store.TraceRecord(id)
	if tr == nil {
		return vtrace.TraceRecord{}, verror.ErrNoExist.Errorf(ctx, "no trace with id %x", id)
	}
	return *tr, nil
}

func (v *vtraceService) AllTraces(ctx *context.T, call s_vtrace.StoreAllTracesServerCall) error {
	// TODO(mattr): Consider changing the store to allow us to iterate through traces
	// when there are many.
	store := vtrace.GetStore(ctx)
	traces := store.TraceRecords()
	for i := range traces {
		if err := call.SendStream().Send(traces[i]); err != nil {
			return err
		}
	}
	return nil
}

func NewVtraceService() interface{} {
	return s_vtrace.StoreServer(&vtraceService{})
}

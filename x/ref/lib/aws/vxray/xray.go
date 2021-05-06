// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vxray

import (
	"net/http"

	"github.com/aws/aws-xray-sdk-go/header"
	"github.com/aws/aws-xray-sdk-go/xray"
	"v.io/v23/context"
)

type traceHeaderKey struct{}

// GetSegment returns the xray segment stored in the context, or nil.
// It uses the same context key as the xray pacakge.
func GetSegment(ctx *context.T) *xray.Segment {
	seg, _ := ctx.Value(xray.ContextKey).(*xray.Segment)
	return seg
}

// WithSegment records the xray segment in the returned context.
// It uses the same context key as the xray pacakge.
func WithSegment(ctx *context.T, seg *xray.Segment) *context.T {
	return context.WithValue(ctx, xray.ContextKey, seg)
}

// GetTraceHeader returns the xray header.Header stored in the current
// context, or nil. It looks for both a *header.Header managed by this
// package as well as the string representation of header.Header managed
// by the xray package.
func GetTraceHeader(ctx *context.T) *header.Header {
	if hdr, _ := ctx.Value(traceHeaderKey{}).(*header.Header); hdr != nil {
		return hdr
	}
	// See if the xray sdk has already inserted a header.
	if tmp := ctx.Value(xray.LambdaTraceHeaderKey); tmp != nil {
		return header.FromString(tmp.(string))
	}
	return nil
}

// WithTraceHeader records the header.Header in the returned context
// as both a pointer managed by this package and the string representation
// managed by the xray package.
func WithTraceHeader(ctx *context.T, hdr *header.Header) *context.T {
	ctx = context.WithValue(ctx, xray.LambdaTraceHeaderKey, hdr.String())
	return context.WithValue(ctx, traceHeaderKey{}, hdr)
}

// WithConfig records the xray.Config in the returned context.
// It uses the same context key as the xray pacakge.
func WithConfig(ctx *context.T, config xray.Config) (*context.T, error) {
	goCtx, err := xray.ContextWithConfig(ctx, config)
	if err != nil {
		ctx.Errorf("failed to create xray context: %v", err)
		return ctx, err
	}
	cfg := xray.GetRecorder(goCtx)
	ctx = context.WithValue(ctx, xray.RecorderContextKey{}, cfg)
	return ctx, nil
}

// GetConfig returns
// It uses the same context key as the xray pacakge.
func GetConfig(ctx *context.T) *xray.Config {
	return ctx.Value(xray.RecorderContextKey{}).(*xray.Config)
}

// MergeHTTPRequestContext merges any xray related metadata such as an
// existing trace header or segment into the returned context.
// This returned context can then be used with all of the vtrace methods
// in this package. MergeHTTPRequestContext should be called by any xray
// managed http.Handler that issues vanadium RPCs or that uses  vtrace.
func MergeHTTPRequestContext(ctx *context.T, req *http.Request) *context.T {
	goctx := req.Context()
	if hdr := goctx.Value(xray.LambdaTraceHeaderKey); hdr != nil {
		ctx = context.WithValue(ctx, xray.LambdaTraceHeaderKey, hdr)
		ctx = context.WithValue(ctx, traceHeaderKey{}, header.FromString(hdr.(string)))
	}
	seg := goctx.Value(xray.ContextKey)
	if seg != nil {
		ctx = context.WithValue(ctx, xray.ContextKey, seg)
	}
	return ctx
}

// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package vxray provides an implementation of vtrace that uses AWS'
// xray service as its backend store. It is intended to be interoperable
// with the AWS xray libraries in the sense that calling the xray libraries
// directly should result in operations that affect the enclosing vtrace spans.
// Similarly, use of the xray http handlers will result in any spans from local
// or remote nested operations being associated with that newly created trace.
//
// vxray is installed on a per-context basis, via the InitXRay function:
//
//	ctx, _ = vxray.InitXRay(ctx,
//	                        v23.GetRuntimeFlags().VtraceFlags,
//	                        xray.Config{ServiceVersion: ""},
//	                        vxray.EC2Plugin(),
//	                        vxray.MergeLogging(true))
//
// Once so initialized any spans created using the returned context
// will be backed by xray. All of the existing vtrace functionality is
// supported by the vxray spans.
//
// xray is http centric and makes sampling decisions when a new trace/segment
// is created. The vtrace.SamplingRequest struct is mapped to the http
// primites that xray expects. In particular:
//
//	SamplingRequest.Local is mapped to http.Request.Host
//	SamplingRequest.Name is mapped to http.Request.URL.Path
//	SamplingRequest.Method is mapped to http.Request.Method
//
// These mappings are used purely for sampling decisions, in addition,
// the name of the top-level span, is treated as the xray ServiceName can
// also be used for sampling decisions. Note that SamplingRequest.Name need
// not be the same as the name of the span. Also note that the Method can
// be any Vanadium method and not one of the pre-defined HTTP methods. This
// appears to work fine with xray since its implementation just compares
// the wildcarded strings. However, the xray console has a limit (10) on the
// length of the methods that can be specified and hence wild cards must be
// used instead; e.g. ResolveSt* rather than ResolveStep. This restriction
// may not apply to sampling rules created programmatically.
//
// The Vanadium RPC runtime creates spans for client and server calls and
// when doing so associates metadata which them that can is both passed
// through to xray as well as, optionally, being interpreted by this package.
// In particular, if the MapToHTTP option is set, (it is the default), then
// some of these annotations (see the comment for MapToHTTP) will translated
// into xray.HTTPData fields.
package vxray

import (
	"net/http"

	"github.com/aws/aws-xray-sdk-go/header"
	"github.com/aws/aws-xray-sdk-go/xray"
	"v.io/v23/context"
)

type traceHeaderKey struct{}

// GetSegment returns the xray segment stored in the context, or nil.
// It uses the same context key as the xray package.
func GetSegment(ctx *context.T) *xray.Segment {
	seg, _ := ctx.Value(xray.ContextKey).(*xray.Segment)
	return seg
}

// WithSegment records the xray segment in the returned context.
// It uses the same context key as the xray package.
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
// It uses the same context key as the xray package.
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
// It uses the same context key as the xray package.
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
		ctx = context.WithValue(ctx, xray.LambdaTraceHeaderKey, hdr) //nolint:revive
		ctx = context.WithValue(ctx, traceHeaderKey{}, header.FromString(hdr.(string)))
	}
	seg := goctx.Value(xray.ContextKey)
	if seg != nil {
		ctx = context.WithValue(ctx, xray.ContextKey, seg)
	}
	return ctx
}

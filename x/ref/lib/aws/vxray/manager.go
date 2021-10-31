// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vxray

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/aws/aws-xray-sdk-go/header"
	"github.com/aws/aws-xray-sdk-go/xray"
	"v.io/v23/context"
	"v.io/v23/uniqueid"
	"v.io/v23/vtrace"

	libvtrace "v.io/x/ref/lib/vtrace"
)

// Manager allows you to create new traces and spans and access the
// vtrace store that
type manager struct {
	options       options
	clusterName   string
	containerID   string
	containerHost string
}

func annotateIfSet(a map[string]interface{}, k, v string) {
	if len(v) == 0 {
		return
	}
	a[k] = v
}

func (m *manager) annotateSegment(seg *xray.Segment, subseg bool) {
	if seg.Annotations == nil {
		return
	}
	if m.options.mapToHTTP && !subseg && (seg.HTTP == nil || seg.HTTP.Request == nil) {
		// Only create fake http fields if they have not already been set.
		hd := seg.GetHTTP()
		req := hd.GetRequest()
		mapAnnotation(seg.Annotations, "name", &req.URL)
		mapAnnotation(seg.Annotations, "method", &req.Method)
		mapAnnotation(seg.Annotations, "clientAddr", &req.ClientIP)
		req.UserAgent = "vanadium"
	}
	annotateIfSet(seg.Annotations, "cluster_name", m.clusterName)
	annotateIfSet(seg.Annotations, "container_id", m.containerID)
	annotateIfSet(seg.Annotations, "container_host", m.containerHost)
}

type xrayspan struct {
	vtrace.Span
	ctx    *context.T
	subseg bool
	mgr    *manager
	seg    *xray.Segment

	// The xray SDK holds onto goroutines until the ctx it is called with is
	// canceled: see https://github.com/aws/aws-xray-sdk-go/issues/51 for how
	// this leads to goroutine leaks. This forces client code of this SDK to
	// ensure that the ctx passed in to it is canceled in a timely manner.
	// For the manager implementation here, a new context is created purely
	// for passing in to the xray SDK and this context is canceled when
	// its Finish method is called. The cancel method is stored in field below.
	// The affected code uses 'xraybugctx' as the variable name of the specifically
	// created context. This is clearly a bug in the xray SDK and if that ever
	// gets fixed then this code can be removed.
	cancel func()
}

func newXraySpan(ctx *context.T, cancel func(), vspan vtrace.Span, mgr *manager, seg *xray.Segment, subseg bool) *xrayspan {
	return &xrayspan{
		Span:   vspan,
		cancel: cancel,
		ctx:    ctx,
		mgr:    mgr,
		seg:    seg,
		subseg: subseg,
	}
}

func (xs *xrayspan) Annotate(msg string) {
	if xs.seg != nil {
		now := time.Now().Format(time.StampMicro)
		xs.seg.AddMetadataToNamespace("vtrace", now, msg)
	}
	xs.Span.Annotate(msg)
}

func (xs *xrayspan) Annotatef(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if xs.seg != nil {
		now := time.Now().Format(time.StampMicro)
		xs.seg.AddMetadataToNamespace("vtrace", now, msg)
	}
	xs.Span.Annotate(msg)
}

func (xs *xrayspan) AnnotateMetadata(key string, value interface{}, indexed bool) error {
	if xs.seg != nil {
		if indexed {
			return xs.seg.AddAnnotation(key, value)
		}
		return xs.seg.AddMetadataToNamespace("vtrace", key, value)
	}
	return xs.Span.AnnotateMetadata(key, value, indexed)
}

func segJSON(seg *xray.Segment) string {
	// Need to look the segment to prevent concurrent read/writes to the
	// annotations etc.
	seg.RLock()
	defer seg.RUnlock()
	out := strings.Builder{}
	enc := json.NewEncoder(&out)
	enc.SetIndent("  ", "  ")
	enc.Encode(seg)
	return out.String()
}

func segStr(seg *xray.Segment) string {
	return fmt.Sprintf("%v: id: %v, parent: %v/%v, trace: %v (%v)", seg.Name, seg.ID, seg.ParentID, seg.ParentSegment.ID, seg.TraceID, getTraceID(seg))
}

func mapAnnotation(annotations map[string]interface{}, key string, to *string) {
	v, ok := annotations[key]
	if !ok {
		return
	}
	if vs, ok := v.(string); ok {
		*to = vs
	}
}

func (xs *xrayspan) Finish(err error) {
	if xs.seg == nil {
		return
	}
	if err != nil {
		xs.seg.AddMetadataToNamespace("vtrace", "error", err.Error())
	}
	xseg := xs.seg
	xs.mgr.annotateSegment(xseg, xs.subseg)
	if xs.subseg {
		xseg.CloseAndStream(err)
	} else {
		xseg.Close(err)
	}
	if xs.ctx.V(1) {
		xs.ctx.Infof("Finish: sampled %v, %v", xseg.Sampled, segJSON(xseg))
	}
	xs.Span.Finish(err)
	if xs.cancel != nil {
		xs.cancel()
	}
}

// WithNewTrace creates a new vtrace context that is not the child of any
// other span.  This is useful when starting operations that are
// disconnected from the activity ctx is performing. For example
// this might be used to start background tasks.
func (m *manager) WithNewTrace(ctx *context.T, name string, sr *vtrace.SamplingRequest) (*context.T, vtrace.Span) {
	st := vtrace.GetStore(ctx)
	if st == nil {
		panic("nil store")
	}

	id, err := uniqueid.Random()
	if err != nil {
		ctx.Errorf("vtrace: couldn't generate Trace Id, debug data may be lost: %v", err)
	}

	newSpan, err := libvtrace.NewSpan(id, id, name, st)
	if err != nil {
		ctx.Error(err)
	}

	if st.Flags(uniqueid.Id{})&vtrace.AWSXRay == 0 {
		// The underlying store is not configured to use xray.
		return vtrace.WithSpan(ctx, newSpan), newSpan
	}

	tid := xray.NewTraceID()
	hdr := &header.Header{
		TraceID: tid,
	}
	ctx = WithTraceHeader(ctx, hdr)
	name = sanitizeName(name)
	sampling := translateSamplingHeader(sr)
	xraybugctx, cancel := context.WithCancel(ctx) // Workaround xray SDK leak.
	_, seg := xray.NewSegmentFromHeader(xraybugctx, name, sampling, hdr)
	ctx = WithSegment(ctx, seg)
	xs := newXraySpan(ctx, cancel, newSpan, m, seg, false)
	return vtrace.WithSpan(ctx, xs), xs
}

var runeMap = map[rune]rune{
	// Allowed runes.
	'_':  '_',
	'.':  '.',
	':':  ':',
	'/':  '/',
	'%':  '%',
	'&':  '&',
	'#':  '#',
	'=':  '=',
	'+':  '+',
	'\\': '\\',
	'-':  '-',
	'@':  '@',

	// Map common unsupported runes to something approximating them.
	'<': ':',
	'>': ':',

	// All other runes are silently dropped.
}

// xray segment names are defined to be:
// letters, numbers, and whitespace, and the
// following symbols: _, ., :, /, %, &, #, =, +, \, -, @
func sanitizeName(name string) string {
	if len(name) == 0 {
		return "-"
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsSpace(r) {
			return r
		}
		if r, ok := runeMap[r]; ok {
			return r
		}
		return -1
	}, name)
}

func getTraceID(seg *xray.Segment) string {
	if len(seg.TraceID) == 0 {
		if seg.ParentSegment != nil {
			return getTraceID(seg.ParentSegment)
		}
		return "none"
	}
	return seg.TraceID
}

// newSegment assumes that ctx is created specifically for passing to the
// xray functions and that this ctx will be be canceled when the span is
// finished with.
func newSegment(ctx *context.T, name string, sampling *http.Request) (seg *xray.Segment, sub bool) {
	sanitized := sanitizeName(name)
	seg = GetSegment(ctx)
	hdr := GetTraceHeader(ctx)
	if seg == nil {
		_, seg = xray.BeginSegmentWithSampling(ctx, sanitized, sampling, hdr)
		ctx.VI(1).Infof("new Top segment: %v", segStr(seg))
	} else {
		_, seg = xray.BeginSubsegment(ctx, sanitized)
		ctx.VI(1).Infof("new Sub segment: %v", segStr(seg))
		sub = true
	}
	return
}

func translateSamplingHeader(sr *vtrace.SamplingRequest) *http.Request {
	if sr == nil {
		return nil
	}
	return &http.Request{
		Method: sr.Method,
		Host:   sr.Local,
		URL:    &url.URL{Path: sr.Name},
	}
}

// WithContinuedTrace creates a span that represents a continuation of
// a trace from a remote server.  name is the name of the new span and
// req contains the parameters needed to connect this span with it's
// trace.
func (m *manager) WithContinuedTrace(ctx *context.T, name string, sr *vtrace.SamplingRequest, req vtrace.Request) (*context.T, vtrace.Span) {
	st := vtrace.GetStore(ctx)
	if st == nil {
		panic("nil store")
	}
	if req.Flags&vtrace.CollectInMemory != 0 {
		st.ForceCollect(req.TraceId, int(req.LogLevel))
	}
	newSpan, err := libvtrace.NewSpan(req.TraceId, req.SpanId, name, st)
	if err != nil {
		ctx.Error(err)
	}

	if req.Flags&vtrace.AWSXRay == 0 || st.Flags(uniqueid.Id{})&vtrace.AWSXRay == 0 {
		// The request or the underlying store is not configured to use xray.
		return vtrace.WithSpan(ctx, newSpan), newSpan
	}

	sampling := translateSamplingHeader(sr)

	name = sanitizeName(name)
	var seg *xray.Segment
	var sub bool
	var cancel func()
	var xrtaybugctx *context.T
	if req.Flags&vtrace.AWSXRay != 0 && len(req.RequestMetadata) > 0 {
		reqHdr := string(req.RequestMetadata)
		hdr := header.FromString(reqHdr)
		ctx = WithTraceHeader(ctx, hdr)
		xrtaybugctx, cancel = context.WithCancel(ctx)
		_, seg = xray.NewSegmentFromHeader(xrtaybugctx, name, sampling, hdr)
		ctx.VI(1).Infof("WithContinuedTrace: new seg from header %v / %v: %v", hdr.TraceID, hdr.ParentID, segStr(seg))
	} else {
		xrtaybugctx, cancel = context.WithCancel(ctx)
		seg, sub = newSegment(xrtaybugctx, name, sampling)
		ctx.VI(1).Infof("WithContinuedTrace: new seg: %v", segStr(seg))
	}

	ctx = WithSegment(ctx, seg)
	xs := newXraySpan(ctx, cancel, newSpan, m, seg, sub)
	return vtrace.WithSpan(ctx, xs), xs
}

// WithNewSpan derives a context with a new Span that can be used to
// trace and annotate operations across process boundaries.
func (m *manager) WithNewSpan(ctx *context.T, name string) (*context.T, vtrace.Span) {
	if curSpan := vtrace.GetSpan(ctx); curSpan != nil {
		if curSpan.Store() == nil {
			panic("nil store")
		}
		newSpan, err := libvtrace.NewSpan(curSpan.Trace(), curSpan.ID(), name, curSpan.Store())
		if err != nil {
			ctx.Error(err)
		}
		xraybugctx, cancel := context.WithCancel(ctx)
		seg, sub := newSegment(xraybugctx, name, nil)
		ctx.VI(1).Infof("WithNewSpan: new seg: %v", segStr(seg))
		ctx = WithSegment(ctx, seg)
		xs := newXraySpan(ctx, cancel, newSpan, m, seg, sub)
		return vtrace.WithSpan(ctx, xs), xs
	}
	ctx.Error("vtrace: creating a new child span from context with no existing span.")
	return m.WithNewTrace(ctx, name, nil)
}

// Request generates a vtrace.Request from the active Span.
func (m *manager) GetRequest(ctx *context.T) vtrace.Request {
	if span := vtrace.GetSpan(ctx); span != nil {
		req := span.Request(ctx)
		if seg := GetSegment(ctx); seg != nil {
			reqHdr := seg.DownstreamHeader()
			req.RequestMetadata = []byte(reqHdr.String())
		}
		return req
	}
	return vtrace.Request{}
}

// Response captures the vtrace.Response for the active Span.
func (m *manager) GetResponse(ctx *context.T) vtrace.Response {
	if span := vtrace.GetSpan(ctx); span != nil {
		return vtrace.Response{
			Flags: span.Store().Flags(span.Trace()),
			Trace: *span.Store().TraceRecord(span.Trace()),
		}
	}
	return vtrace.Response{}
}

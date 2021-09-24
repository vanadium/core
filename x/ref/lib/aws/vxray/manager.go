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
	xs.ctx.VI(1).Infof("Finish: sampled %v, %v", xseg.Sampled, segJSON(xseg))
	xs.Span.Finish(err)
}

// WithNewTrace creates a new vtrace context that is not the child of any
// other span.  This is useful when starting operations that are
// disconnected from the activity ctx is performing. For example
// this might be used to start background tasks.
func (m *manager) WithNewTrace(ctx *context.T, name string, sr *vtrace.SamplingRequest) (*context.T, vtrace.Span) {
	id, err := uniqueid.Random()
	if err != nil {
		ctx.Errorf("vtrace: couldn't generate Trace Id, debug data may be lost: %v", err)
	}
	newSpan, err := libvtrace.NewSpan(id, id, name, vtrace.GetStore(ctx))
	if err != nil {
		ctx.Error(err)
	}
	tid := xray.NewTraceID()
	hdr := &header.Header{
		TraceID: tid,
	}
	ctx = WithTraceHeader(ctx, hdr)
	name = sanitizeName(name)
	sampling := translateSamplingHeader(sr)
	_, seg := xray.NewSegmentFromHeader(ctx, name, sampling, hdr)
	ctx = WithSegment(ctx, seg)
	xs := &xrayspan{Span: newSpan, ctx: ctx, mgr: m, seg: seg}
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

func newSegment(ctx *context.T, name string, sampling *http.Request) (seg *xray.Segment, sub bool) {
	sanitized := sanitizeName(name)
	seg = GetSegment(ctx)
	hdr := GetTraceHeader(ctx)
	// Creating a new context and then ensuring that it is canceled is purely
	// to work around the goroutine leak in BeginSegmentWithSampling documented
	// in: https://github.com/aws/aws-xray-sdk-go/issues/51
	// Calling cancel here ensures that the goroutine created by
	// BeginSegmentWithSampling exits now, rather than at some later point
	// when the ctx passed in to newSegment is called. This is clearly a bug
	// in the xray SDK and if that ever gets fixed then this code can be
	// removed.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
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

	sampling := translateSamplingHeader(sr)

	name = sanitizeName(name)
	var seg *xray.Segment
	var sub bool
	if req.Flags&vtrace.AWSXRay != 0 && len(req.RequestMetadata) > 0 {
		reqHdr := string(req.RequestMetadata)
		hdr := header.FromString(reqHdr)
		ctx = WithTraceHeader(ctx, hdr)
		_, seg = xray.NewSegmentFromHeader(ctx, name, sampling, hdr)
		ctx.VI(1).Infof("WithContinuedTrace: new seg from header %v / %v: %v", hdr.TraceID, hdr.ParentID, segStr(seg))
	} else {
		seg, sub = newSegment(ctx, name, sampling)
		ctx.VI(1).Infof("WithContinuedTrace: new seg: %v", segStr(seg))
	}

	ctx = WithSegment(ctx, seg)
	xs := &xrayspan{Span: newSpan, ctx: ctx, mgr: m, seg: seg, subseg: sub}
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
		seg, sub := newSegment(ctx, name, nil)
		ctx.VI(1).Infof("WithNewSpan: new seg: %v", segStr(seg))
		ctx = WithSegment(ctx, seg)
		xs := &xrayspan{Span: newSpan, ctx: ctx, mgr: m, seg: seg, subseg: sub}
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

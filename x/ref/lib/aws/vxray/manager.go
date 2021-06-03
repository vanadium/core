// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vxray

import (
	"encoding/json"
	"fmt"
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
	mapToHTTP bool
}

type xrayspan struct {
	vtrace.Span
	subseg    bool
	mapToHTTP bool
	seg       *xray.Segment
}

// TODO(cnicolaou): figure out why annotations are not being recorded
// on AWS.
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
		xs.seg.AddMetadata("error", err.Error())
	}
	xseg := xs.seg
	if an := xseg.Annotations; !xs.subseg && xs.mapToHTTP && an != nil {
		hd := xseg.GetHTTP()
		req := hd.GetRequest()
		mapAnnotation(an, "name", &req.URL)
		mapAnnotation(an, "method", &req.Method)
		mapAnnotation(an, "clientAddr", &req.ClientIP)
		req.UserAgent = "vanadium"
	}
	if xs.subseg {
		xseg.CloseAndStream(err)
	} else {
		xseg.Close(err)
	}
	xs.Span.Finish(err)
}

// WithNewTrace creates a new vtrace context that is not the child of any
// other span.  This is useful when starting operations that are
// disconnected from the activity ctx is performing. For example
// this might be used to start background tasks.
func (m manager) WithNewTrace(ctx *context.T, name string) (*context.T, vtrace.Span) {
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
	_, seg := xray.NewSegmentFromHeader(ctx, name, nil, hdr)
	ctx = WithSegment(ctx, seg)
	xs := &xrayspan{Span: newSpan, mapToHTTP: m.mapToHTTP, seg: seg}
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
	//'"': '-',
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

func segStr(seg *xray.Segment) string {
	return fmt.Sprintf("%v: id: %v, parent: %v/%v, trace: %v (%v)", seg.Name, seg.ID, seg.ParentID, seg.ParentSegment.ID, seg.TraceID, getTraceID(seg))
}

func newSegment(ctx *context.T, name string) (seg *xray.Segment, sub bool) {
	sanitized := sanitizeName(name)
	seg = GetSegment(ctx)
	hdr := GetTraceHeader(ctx)
	if seg == nil {
		// TODO(cnicolaou): BeginSegmentWithSampling expects an
		// *http.Request to use for sampling decisions, but the vtrace
		// API does not allow for specifying metadata until after the
		// segment is created. One option is to delay creating
		// the xray segment until the span is finished, but that
		// seems complicated given the nested structure of xray Segments
		// and vtrace spans, especially given that xray may be used
		// directly by libraries and server code. Another option is to
		// create a new segment that's a copy of the existing one, including
		// updating all subsegments, and evaluate that to see if it is to
		// be sampled. If the code is left as is, then only the ServiceName
		// can be used for sampling decisions. The TODO is to figure out what
		// to do, if anything.
		_, seg = xray.BeginSegmentWithSampling(ctx, sanitized, nil, hdr)
		ctx.VI(1).Infof("new Top segment: %v", segStr(seg))
	} else {
		_, seg = xray.BeginSubsegment(ctx, sanitized)
		ctx.VI(1).Infof("new Sub segment: %v", segStr(seg))
		sub = true
	}
	return
}

// WithContinuedTrace creates a span that represents a continuation of
// a trace from a remote server.  name is the name of the new span and
// req contains the parameters needed to connect this span with it's
// trace.
func (m manager) WithContinuedTrace(ctx *context.T, name string, req vtrace.Request) (*context.T, vtrace.Span) {
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

	name = sanitizeName(name)
	var seg *xray.Segment
	var sub bool
	if req.Flags&vtrace.AWSXRay != 0 && len(req.RequestMetadata) > 0 {
		reqHdr := string(req.RequestMetadata)
		hdr := header.FromString(reqHdr)
		ctx = WithTraceHeader(ctx, hdr)
		_, seg = xray.NewSegmentFromHeader(ctx, name, nil, hdr)
		ctx.VI(1).Infof("WithContinuedTrace: new seg from header %v / %v: %v", hdr.TraceID, hdr.ParentID, segStr(seg))
	} else {
		seg, sub = newSegment(ctx, name)
		ctx.VI(1).Infof("WithContinuedTrace: new seg: %v", segStr(seg))
	}

	ctx = WithSegment(ctx, seg)
	xs := &xrayspan{Span: newSpan, mapToHTTP: m.mapToHTTP, seg: seg, subseg: sub}
	return vtrace.WithSpan(ctx, xs), xs
}

// WithNewSpan derives a context with a new Span that can be used to
// trace and annotate operations across process boundaries.
func (m manager) WithNewSpan(ctx *context.T, name string) (*context.T, vtrace.Span) {
	if curSpan := vtrace.GetSpan(ctx); curSpan != nil {
		if curSpan.Store() == nil {
			panic("nil store")
		}
		newSpan, err := libvtrace.NewSpan(curSpan.Trace(), curSpan.ID(), name, curSpan.Store())
		if err != nil {
			ctx.Error(err)
		}
		seg, sub := newSegment(ctx, name)
		ctx.VI(1).Infof("WithNewSpan: new seg: %v", segStr(seg))
		ctx = WithSegment(ctx, seg)
		xs := &xrayspan{Span: newSpan, mapToHTTP: m.mapToHTTP, seg: seg, subseg: sub}
		return vtrace.WithSpan(ctx, xs), xs
	}
	ctx.Error("vtrace: creating a new child span from context with no existing span.")
	return m.WithNewTrace(ctx, name)
}

// Request generates a vtrace.Request from the active Span.
func (m manager) GetRequest(ctx *context.T) vtrace.Request {
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
func (m manager) GetResponse(ctx *context.T) vtrace.Response {
	if span := vtrace.GetSpan(ctx); span != nil {
		return vtrace.Response{
			Flags: span.Store().Flags(span.Trace()),
			Trace: *span.Store().TraceRecord(span.Trace()),
		}
	}
	return vtrace.Response{}
}

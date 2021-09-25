package vxray_test

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/aws/aws-xray-sdk-go/strategy/sampling"
	"github.com/aws/aws-xray-sdk-go/xray"
	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/vtrace"
	"v.io/x/ref/lib/aws/vxray"
	"v.io/x/ref/lib/flags"
	libvtrace "v.io/x/ref/lib/vtrace"

	_ "v.io/x/ref/runtime/factories/generic"
)

type emitter struct {
	segs []*xray.Segment
}

func (e *emitter) Emit(seg *xray.Segment) {
	e.segs = append(e.segs, seg)
}

func (e *emitter) RefreshEmitterWithAddress(raddr *net.UDPAddr) {}

const samplingRulesJSON = `
{
	"version": 2,
	"rules": [
	  {
		"description": "drop",
		"host": "myhost",
		"http_method": "method",
		"url_path": "path",
		"fixed_target": 0,
		"rate": 0.00
	  }
	],
	"default": {
	  "fixed_target": 100,
	  "rate": 1.00
	}
  }
`

var samplingRules sampling.Strategy

func init() {
	var err error
	samplingRules, err = sampling.NewLocalizedStrategyFromJSONBytes([]byte(samplingRulesJSON))
	if err != nil {
		panic(fmt.Sprintf("failed to parse %s: %s", samplingRulesJSON, err))
	}
}

func (e *emitter) validateSegments(names []string) error {
	if got, want := len(e.segs), len(names); got != want {
		return fmt.Errorf("# segments: got %v, want %v", got, want)
	}
	for i, name := range names {
		seg := e.segs[i]
		if got, want := seg.Name, name; got != want {
			return fmt.Errorf("%v: got %v, want %v", i, got, want)
		}
	}
	return nil
}

func initXRay(t *testing.T, ctx *context.T, captured xray.Emitter) *context.T {
	xrayFlags := flags.VtraceFlags{
		SampleRate:     1.0,
		EnableAWSXRay:  true,
		CacheSize:      1024,
		DumpOnShutdown: false,
	}
	ctx, err := vxray.InitXRay(ctx,
		xrayFlags,
		xray.Config{Emitter: captured, SamplingStrategy: samplingRules},
		vxray.WithNewStore(xrayFlags),
		vxray.MergeLogging(true))
	if err != nil {
		t.Fatal(err)
	}
	return ctx
}

func TestSimple(t *testing.T) {
	ctx, shutdown := v23.Init()
	defer shutdown()

	captured := &emitter{}
	ctx = initXRay(t, ctx, captured)

	// Note that all sampling rules will match against an empty field
	// so a method must be specified to avoid the 'drop' rule above. This
	// is an idiosyncrasy of localized rules which do not consider
	// the service_name - i.e. the name of the trace. Centralized rules
	// do consider the service_name.
	sr := &vtrace.SamplingRequest{
		Name: "leave-me-alone", // Name maps to url path.
	}

	ctx, s1 := vtrace.WithNewTrace(ctx, "span1", sr)
	ctx, s2 := vtrace.WithNewSpan(ctx, "span1.2")
	s2.Finish(nil)
	s1.Finish(nil)

	// Spans are emitted in the order that they are finished.
	if err := captured.validateSegments([]string{"span1.2", "span1"}); err != nil {
		t.Fatal(err)
	}

	captured.segs = nil
	ctx, s1 = vtrace.WithNewTrace(ctx, "span1", sr)
	s1.Finish(fmt.Errorf("oops"))
	if err := captured.validateSegments([]string{"span1"}); err != nil {
		t.Fatal(err)
	}
	if got, want := captured.segs[0].Fault, true; got != want {
		t.Log(captured.segs[0])
		t.Errorf("got %v, want %v", got, want)
	}

	captured.segs = nil
	sr.Name = "path"
	sr.Method = "method"
	ctx, s3 := vtrace.WithNewTrace(ctx, "span1", sr)
	_, s4 := vtrace.WithNewSpan(ctx, "span1.2")
	s4.Finish(nil)
	s3.Finish(nil)

	if err := captured.validateSegments([]string{}); err != nil {
		t.Fatal(err)
	}

}

type simple struct{}

func (s *simple) Ping(ctx *context.T, _ rpc.ServerCall) (string, error) {
	//defer fmt.Printf("ping: ctx %p\n", ctx)
	//time.Sleep(500 * time.Millisecond)
	return "pong", nil
}

func issueCall(ctx *context.T, name string) (*context.T, vtrace.Span, error) {
	sr := &vtrace.SamplingRequest{
		Name: "leave-me-alone",
	}
	ctx, span := vtrace.WithNewTrace(ctx, "issueCall", sr)
	defer span.Finish(nil)

	client := v23.GetClient(ctx)
	call, err := client.StartCall(ctx, name, "Ping", nil)
	if err != nil {
		return ctx, span, fmt.Errorf("unexpected error: %s", err)
	}
	response := ""
	if err := call.Finish(&response); err != nil {
		return ctx, span, fmt.Errorf("unexpected error: %s", err)
	}
	if got, want := response, "pong"; got != want {
		return ctx, span, fmt.Errorf("got %q, want %q", got, want)
	}
	return ctx, span, nil
}

func checkSeg(t *testing.T, seg *xray.Segment, name string, annotations ...string) {
	if got, want := seg.Name, name; got != want {
		t.Errorf("name: got %v, want %v", got, want)
	}

	for _, key := range annotations {
		if _, ok := seg.Annotations[key]; !ok {
			t.Errorf("seg %v is missing annotation: %v", name, key)
		}
	}
}

func checkTraceRecord(t *testing.T, tr *vtrace.TraceRecord, names ...string) {
	if got, want := len(tr.Spans), len(names); got != want {
		t.Errorf("# spans: got %v, want %v", got, want)
	}

	for _, name := range names {
		found := false
		for _, span := range tr.Spans {
			if span.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("failed to find span name: %v", name)
			for _, s := range tr.Spans {
				t.Logf("span: %v", s.Name)
			}
		}
	}

}

func TestWithRPC(t *testing.T) {
	ctx, shutdown := v23.Init()
	defer shutdown()

	captured := &emitter{}

	clientCtx := initXRay(t, ctx, captured)
	serverCtx := initXRay(t, ctx, captured)

	_, server, err := v23.WithNewServer(serverCtx, "", &simple{}, security.AllowEveryone())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	name := server.Status().Endpoints[0].Name()

	clientCtx, _, err = issueCall(clientCtx, name)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := len(captured.segs), 4; got != want {
		t.Fatalf("# segments: got %v, want %v", got, want)
	}

	checkSeg(t, captured.segs[0], ":rpc.Client:tryConnectToServer",
		"isClient", "name", "method")

	checkSeg(t, captured.segs[1], ".Ping",
		"isServer", "name", "method", "clientAddr",
		"remoteBlessings", "remotePublicKey")

	checkSeg(t, captured.segs[2],
		fmt.Sprintf(":rpc.Client:%s.Ping", name),
		"isClient", "name", "method")

	checkSeg(t, captured.segs[3], "issueCall")

	resp := vtrace.GetResponse(clientCtx)
	checkTraceRecord(t, &resp.Trace,
		fmt.Sprintf("<rpc.Client>%q.Ping", name),
		"<rpc.Client>tryConnectToServer", "issueCall", ".Ping")

	// Run without vxray.
	captured.segs = nil

	store, _ := libvtrace.NewStore(flags.VtraceFlags{
		SampleRate:     1,
		DumpOnShutdown: false,
		CacheSize:      1024,
		EnableAWSXRay:  false,
	})

	clientCtx = vtrace.WithStore(ctx, store)

	clientCtx, span, err := issueCall(clientCtx, name)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := len(captured.segs), 0; got != want {
		for i, s := range captured.segs {
			t.Logf("segments: %v: %v", i, s)
		}
		t.Errorf("# segments: got %v, want %v", got, want)
	}

	trace := vtrace.GetStore(clientCtx).TraceRecord(span.Trace())
	if got, want := len(trace.Spans), 4; got != want {
		for i, s := range trace.Spans {
			t.Logf("spans: %v: %v", i, s)
		}
		t.Fatalf("# spans: got %v, want %v", got, want)
	}

	resp = vtrace.GetResponse(clientCtx)
	checkTraceRecord(t, &resp.Trace,
		fmt.Sprintf("<rpc.Client>%q.Ping", name),
		"<rpc.Client>tryConnectToServer", "issueCall", ".Ping")
}

func TestWithXRayHTTPHandler(t *testing.T) {
	ctx, shutdown := v23.Init()
	defer shutdown()

	captured := &emitter{}

	clientCtx := initXRay(t, ctx, captured)
	serverCtx := initXRay(t, ctx, captured)

	_, server, err := v23.WithNewServer(serverCtx, "", &simple{}, security.AllowEveryone())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	name := server.Status().Endpoints[0].Name()

	xrayHandler := xray.Handler(
		xray.NewFixedSegmentNamer("http.echo.client"),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _, err = issueCall(clientCtx, name)
			if err != nil {
				t.Fatal(err)
			}

		}))

	ts := httptest.NewServer(xrayHandler)
	defer ts.Close()

	res, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if res.StatusCode != 200 {
		t.Fatalf("get failed: http status code %v", res.StatusCode)
	}

	if got, want := len(captured.segs), 5; got != want {
		t.Fatalf("# segments: got %v, want %v", got, want)
	}

	checkSeg(t, captured.segs[0], ":rpc.Client:tryConnectToServer",
		"isClient", "name", "method")

	checkSeg(t, captured.segs[1], ".Ping",
		"isServer", "name", "method", "clientAddr",
		"remoteBlessings", "remotePublicKey")

	checkSeg(t, captured.segs[2],
		fmt.Sprintf(":rpc.Client:%s.Ping", name),
		"isClient", "name", "method")

	checkSeg(t, captured.segs[3], "issueCall")

	checkSeg(t, captured.segs[4], "http.echo.client")

}

func TestGoRoutineLeak(t *testing.T) {
	// https://github.com/aws/aws-xray-sdk-go/issues/51 outlines how
	// the xray SDK can leak goroutines. The vxray package works around
	// this leak and this test is intended to verify that it does so.
	ctx, shutdown := v23.Init()
	defer shutdown()

	captured := &emitter{}

	initialGoRoutines := runtime.NumGoroutine()
	xrayctx := initXRay(t, ctx, captured)
	iterations := 1000
	for i := 0; i < iterations; i++ {
		sr := &vtrace.SamplingRequest{
			Name: fmt.Sprintf("%v", i),
		}
		_, span := vtrace.WithNewTrace(xrayctx, "test", sr)
		span.Finish(nil)
	}
	runtime.Gosched()
	currentGoRoutines := runtime.NumGoroutine() - initialGoRoutines

	if currentGoRoutines > iterations/2 {
		t.Fatalf("%v running goroutines seems too high", currentGoRoutines)
	}
}

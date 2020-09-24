// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package browseserver provides a web interface that can be used to interact with the
// vanadium debug interface.
package browseserver

import (
	"bytes"
	gocontext "context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security"
	"v.io/v23/services/logreader"
	"v.io/v23/services/stats"
	svtrace "v.io/v23/services/vtrace"
	"v.io/v23/uniqueid"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/v23/vtrace"
	"v.io/x/ref/services/internal/pproflib"
	s_stats "v.io/x/ref/services/stats"
)

//go:generate ./gen_assets.sh

const browseProfilesPath = "/profiles"

const (
	allTraceTmpl  = "alltrace.html"
	blessingsTmpl = "blessings.html"
	chromeTmpl    = "chrome.html"
	globTmpl      = "glob.html"
	logsTmpl      = "logs.html"
	profilesTmpl  = "profiles.html"
	resolveTmpl   = "resolve.html"
	statsTmpl     = "stats.html"
	vtraceTmpl    = "vtrace.html"
)

// Serve serves the debug interface over http.  An HTTP server is started (serving at httpAddr), its
// various handlers make rpc calls to the given name to gather debug information.  If log is true
// we additionally log debug information for these rpc requests.  Timeout defines the timeout for the
// rpc calls.  The HTTPServer will run until the passed context is canceled.
func Serve(ctx *context.T, httpAddr, name string, timeout time.Duration, log bool, assetDir string) error {
	mux, err := CreateServeMux(ctx, timeout, log, assetDir, "")
	if err != nil {
		return err
	}
	if len(httpAddr) == 0 {
		httpAddr = "127.0.0.1:0"
	}
	ln, err := net.Listen("tcp", httpAddr)
	if err != nil {
		return err
	}
	go func() {
		url := "http://" + ln.Addr().String() + "/?n=" + url.QueryEscape(name)
		fmt.Printf("Visit %s and Ctrl-C to quit\n", url)
		// Open the browser if we can
		switch runtime.GOOS {
		case "linux":
			exec.Command("xdg-open", url).Start() //nolint:errcheck
		case "darwin":
			exec.Command("open", url).Start() //nolint:errcheck
		}

		<-ctx.Done()
		ln.Close()
	}()
	return http.Serve(ln, mux)
}

// CreateServeMux returns a ServeMux object that has handlers set up.
func CreateServeMux(ctx *context.T, timeout time.Duration, log bool, assetDir, urlPrefix string) (*http.ServeMux, error) {
	mux := http.NewServeMux()
	h, err := newHandler(ctx, timeout, log, assetDir, urlPrefix)
	if err != nil {
		return nil, err
	}
	mux.Handle("/", &resolveHandler{h})
	mux.Handle("/name", &resolveHandler{h})
	mux.Handle("/stats", &statsHandler{h})
	mux.Handle("/blessings", &blessingsHandler{h})
	mux.Handle("/logs", &logsHandler{h})
	mux.Handle("/glob", &globHandler{h})
	mux.Handle(browseProfilesPath, &profilesHandler{h})
	mux.Handle(browseProfilesPath+"/", &profilesHandler{h})
	mux.Handle("/vtraces", &allTracesHandler{h})
	mux.Handle("/vtrace", &vtraceHandler{h})
	mux.Handle("/favicon.ico", http.NotFoundHandler())

	return mux, nil
}

type handler struct {
	ctx       *context.T
	timeout   time.Duration
	log       bool
	urlPrefix string
	file      func(name string) ([]byte, error)
	cacheMap  map[string]*template.Template
	funcs     template.FuncMap
}

func newHandler(ctx *context.T, timeout time.Duration, log bool, assetDir, urlPrefix string) (*handler, error) {
	h := &handler{
		ctx:       ctx,
		timeout:   timeout,
		log:       log,
		urlPrefix: urlPrefix,
	}
	colors := []string{"red", "blue", "green"}
	pos := 0
	h.funcs = template.FuncMap{
		"verrorID":           verror.ErrorID,
		"unmarshalPublicKey": security.UnmarshalPublicKey,
		"endpoint": func(n string) (naming.Endpoint, error) {
			if naming.Rooted(n) {
				n, _ = naming.SplitAddressName(n)
			}
			return naming.ParseEndpoint(n)
		},
		"endpointName": func(ep naming.Endpoint) string { return ep.Name() },
		"goValueFromVOM": func(v *vom.RawBytes) interface{} {
			var ret interface{}
			if err := v.ToValue(&ret); err != nil {
				panic(err)
			}
			return ret
		},
		"nextColor": func() string {
			c := colors[pos]
			pos = (pos + 1) % len(colors)
			return c
		},
	}

	if assetDir == "" {
		h.file = Asset
		h.cacheMap = make(map[string]*template.Template)
		all := []string{chromeTmpl, allTraceTmpl, blessingsTmpl, globTmpl,
			logsTmpl, profilesTmpl, resolveTmpl, statsTmpl, vtraceTmpl}
		for _, tmpl := range all {
			if _, err := h.template(ctx, tmpl); err != nil {
				return nil, err
			}
		}
	} else {
		h.file = func(name string) ([]byte, error) {
			return ioutil.ReadFile(filepath.Join(assetDir, name))
		}
	}
	return h, nil
}

func (h *handler) template(ctx *context.T, name string) (t *template.Template, err error) {
	if t = h.cacheMap[name]; t != nil {
		return t, nil
	}
	if name == chromeTmpl {
		t = template.New(name).Funcs(h.funcs)
	} else {
		if t, err = h.template(ctx, chromeTmpl); err != nil {
			return nil, err
		}
		if t, err = t.Clone(); err != nil {
			return nil, err
		}
	}
	src, err := h.file(name)
	if err != nil {
		return nil, err
	}
	if _, err = t.Parse(string(src)); err != nil {
		return nil, err
	}
	return t, nil
}

func (h *handler) execute(ctx *context.T, w http.ResponseWriter, r *http.Request, tmplName string, args interface{}) {
	if h.log {
		ctx.Infof("DEBUG: %q -- %+v", r.URL, args)
	}
	t, err := h.template(ctx, tmplName)
	if err != nil {
		fmt.Fprintf(w, "ERROR:%v", err)
		ctx.Errorf("Error parsing template %q: %v", tmplName, err)
		return
	}
	if err := t.Execute(w, args); err != nil {
		fmt.Fprintf(w, "ERROR:%v", err)
		ctx.Errorf("Error executing template %q: %v", tmplName, err)
		return
	}
}

func (h *handler) withTimeout(ctx *context.T) (*context.T, func()) {
	return context.WithTimeout(ctx, h.timeout)
}

type resolveHandler struct{ *handler }

func (h *resolveHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("n")
	var suffix string
	ctx, tracer := newTracer(h.ctx)
	timeoutCtx, cancel := h.withTimeout(ctx)
	defer cancel()
	m, err := v23.GetNamespace(ctx).Resolve(timeoutCtx, name)
	if m != nil {
		suffix = m.Name
	}
	args := struct {
		ServerName  string
		CommandLine string
		Vtrace      *Tracer
		MountEntry  *naming.MountEntry
		Error       error
	}{
		ServerName:  strings.TrimSuffix(name, suffix),
		CommandLine: fmt.Sprintf("debug resolve %q", name),
		Vtrace:      tracer,
		MountEntry:  m,
		Error:       err,
	}
	h.execute(h.ctx, w, r, "resolve.html", args)
}

type blessingsHandler struct{ *handler }

func (h *blessingsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("n")
	ctx, tracer := newTracer(h.ctx)
	timeoutCtx, cancel := h.withTimeout(ctx)
	call, err := v23.GetClient(ctx).StartCall(timeoutCtx, name, "DoNotReallyCareAboutTheMethod", nil)
	args := struct {
		ServerName        string
		CommandLine       string
		Vtrace            *Tracer
		Error             error
		Blessings         security.Blessings
		Recognized        []string
		CertificateChains [][]security.Certificate
	}{
		ServerName:  name,
		Vtrace:      tracer,
		CommandLine: fmt.Sprintf("vrpc identify %q", name),
		Error:       err,
	}
	if call != nil {
		args.Recognized, args.Blessings = call.RemoteBlessings()
		args.CertificateChains = security.MarshalBlessings(args.Blessings).CertificateChains
		// Don't actually care about the RPC, so don't bother waiting on the Finish.
		cancel()
		defer func() {
			go call.Finish() //nolint:errcheck
		}()
	} else {
		cancel()
	}
	h.execute(h.ctx, w, r, "blessings.html", args)
}

type statsHandler struct{ *handler }

func (h *statsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		server      = r.FormValue("n")
		stat        = r.FormValue("s")
		prefix      = naming.Join(server, "__debug", "stats")
		name        = naming.Join(prefix, stat)
		ctx, tracer = newTracer(h.ctx)
		hasValue    = true
	)
	timeoutCtx, cancel := h.withTimeout(ctx)
	defer cancel()
	v, err := stats.StatsClient(name).Value(timeoutCtx)
	if errors.Is(err, verror.ErrNoExist) {
		// Stat does not exist as a value, that's okay.
		err = nil
		hasValue = false
	}
	var children []string
	var childrenErrors []error
	timeoutCtx2, cancel2 := h.withTimeout(ctx)
	defer cancel2()
	if glob, globErr := v23.GetNamespace(ctx).Glob(timeoutCtx2, naming.Join(name, "*")); globErr == nil {
		for e := range glob {
			switch e := e.(type) {
			case *naming.GlobReplyEntry:
				children = append(children, strings.TrimPrefix(e.Value.Name, prefix))
			case *naming.GlobReplyError:
				childrenErrors = append(childrenErrors, e.Value.Error)
			}
		}
		if len(children) == 1 && !hasValue {
			// Single child, save an extra click
			redirect, err := url.Parse(r.URL.String())
			if err == nil {
				q := redirect.Query()
				q.Set("n", server)
				q.Set("s", children[0])
				redirect.RawQuery = q.Encode()
				ctx.Infof("Redirecting from %v to %v", r.URL, redirect)
				redirectString := redirect.String()
				if h.urlPrefix != "" {
					redirectString = fmt.Sprintf("%s/%s", h.urlPrefix, redirectString)
				}
				http.Redirect(w, r, redirectString, http.StatusTemporaryRedirect)
				return
			}
		}
	} else if err == nil {
		err = globErr
	}
	timeSeriesData, tsErr := h.GetTimeSeriesData(server, children, timeoutCtx)
	if tsErr != nil {
		err = tsErr
	}

	args := struct {
		ServerName     string
		CommandLine    string
		Vtrace         *Tracer
		StatName       string
		Value          *vom.RawBytes
		Children       []string
		ChildrenErrors []error
		Globbed        bool
		TimeSeriesData string
		Error          error
	}{
		ServerName:     server,
		Vtrace:         tracer,
		StatName:       strings.TrimPrefix(stat, "/"),
		Value:          v,
		Error:          err,
		Children:       children,
		ChildrenErrors: childrenErrors,
		TimeSeriesData: timeSeriesData,
		Globbed:        len(children)+len(childrenErrors) > 0,
	}
	if hasValue {
		args.CommandLine = fmt.Sprintf("debug stats watch %q", name)
	} else {
		args.CommandLine = fmt.Sprintf("debug glob %q", naming.Join(name, "*"))
	}
	h.execute(h.ctx, w, r, "stats.html", args)
}

func (h *statsHandler) GetTimeSeriesData(server string, children []string, ctx *context.T) (string, error) {
	// duration -> [[t0, v0], [t1, v1], ... , [tn, vn]]
	timeSeriesData := map[string][][]string{}
	for _, child := range children {
		// Read data from time series related children.
		if strings.Contains(child, "/timeseries") {
			n := fmt.Sprintf("%s%s", naming.Join(server, "__debug", "stats"), child)
			v, err := stats.StatsClient(n).Value(ctx)
			if err != nil && !errors.Is(err, verror.ErrNoExist) {
				return "", err
			}
			var value interface{}
			if err := v.ToValue(&value); err != nil {
				return "", err
			}
			ts, ok := value.(s_stats.TimeSeries)
			if !ok {
				return "", fmt.Errorf("%q doesn't contain time series data", child)
			}
			d := [][]string{}
			startTime := ts.StartTime.Unix()
			resolutionInSeconds := int64(ts.Resolution / time.Second)
			for i, v := range ts.Values {
				curTime := startTime + int64(i)*resolutionInSeconds
				d = append(d, []string{fmt.Sprintf("%d", curTime), fmt.Sprintf("%d", v)})
			}
			parts := strings.Split(child, "/")
			duration := strings.TrimPrefix(parts[len(parts)-1], "timeseries")
			timeSeriesData[duration] = d
		}
	}
	if len(timeSeriesData) == 0 {
		return "", nil
	}
	b, err := json.Marshal(timeSeriesData)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type logsHandler struct{ *handler }

func (h *logsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) { //nolint:gocyclo
	var (
		server = r.FormValue("n")
		log    = r.FormValue("l")
		prefix = naming.Join(server, "__debug", "logs")
		name   = naming.Join(prefix, log)
		path   = strings.TrimPrefix(r.URL.Path, "/")
		list   = func() bool {
			for _, a := range r.Header[http.CanonicalHeaderKey("Accept")] {
				if a == "text/event-stream" {
					return true
				}
			}
			return false
		}()
		ctx, _ = newTracer(h.ctx)
	)
	// The logs handler streams result to the web browser because there
	// have been cases where there are ~1 million log files, so doing this
	// streaming thing will make the UI more responsive.
	//
	// For the same reason, avoid setting a timeout.
	if len(log) == 0 && list {
		w.Header().Add("Content-Type", "text/event-stream")
		glob, err := v23.GetNamespace(ctx).Glob(ctx, naming.Join(name, "*"))
		if err != nil {
			writeErrorEvent(w, err)
			return
		}
		flusher, _ := w.(http.Flusher)
		for e := range glob {
			switch e := e.(type) {
			case *naming.GlobReplyEntry:
				logfile := strings.TrimPrefix(e.Value.Name, prefix+"/")
				writeEvent(w, fmt.Sprintf(
					`<a href="%s?n=%s&l=%s">%s</a>`,
					path,
					url.QueryEscape(server),
					url.QueryEscape(logfile),
					html.EscapeString(logfile)))
			case *naming.GlobReplyError:
				writeErrorEvent(w, e.Value.Error)
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		return
	}
	if len(log) == 0 {
		args := struct {
			ServerName  string
			CommandLine string
			Vtrace      *Tracer
		}{
			ServerName:  server,
			CommandLine: fmt.Sprintf("debug glob %q", naming.Join(name, "*")),
		}
		h.execute(h.ctx, w, r, "logs.html", args)
		return
	}
	w.Header().Add("Content-Type", "text/plain")
	stream, err := logreader.LogFileClient(name).ReadLog(ctx, 0, logreader.AllEntries, true)
	if err != nil {
		fmt.Fprintf(w, "ERROR(%v): %v\n", verror.ErrorID(err), err)
		return
	}
	var (
		entries  = make(chan logreader.LogEntry)
		abortRPC = make(chan bool)
		errch    = make(chan error, 1) // At most one write on this channel, avoid blocking any goroutines
	)
	go func() {
		// writes to: entries, errch
		// reads from: abortRPC
		defer stream.Finish() //nolint:errcheck
		defer close(entries)
		iterator := stream.RecvStream()
		for iterator.Advance() {
			select {
			case entries <- iterator.Value():
			case <-abortRPC:
				return
			}
		}
		if err := iterator.Err(); err != nil {
			errch <- err
		}
	}()
	goctx, cancel := gocontext.WithCancel(r.Context())
	defer cancel()
	// reads from: entries, errch, cancel
	// writes to: abortRPC
	defer close(abortRPC)
	for {
		select {
		case e, more := <-entries:
			if !more {
				return
			}
			fmt.Fprintln(w, e.Line)
		case err := <-errch:
			fmt.Fprintf(w, "ERROR(%v): %v\n", verror.ErrorID(err), err)
			return
		case <-goctx.Done():
			return
		}
	}
}

type globHandler struct{ *handler }

func (h *globHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	type entry struct {
		Suffix string
		Error  error
	}
	var (
		server      = r.FormValue("n")
		suffix      = r.FormValue("s")
		pattern     = naming.Join(server, suffix, "*")
		ctx, tracer = newTracer(h.ctx)
		entries     []entry
	)
	timeoutCtx, cancel := h.withTimeout(ctx)
	defer cancel()
	ch, err := v23.GetNamespace(ctx).Glob(timeoutCtx, pattern)
	if err != nil {
		entries = append(entries, entry{Error: err})
	}
	if ch != nil {
		for e := range ch {
			switch e := e.(type) {
			case *naming.GlobReplyEntry:
				entries = append(entries, entry{Suffix: strings.TrimPrefix(e.Value.Name, server)})
			case *naming.GlobReplyError:
				entries = append(entries, entry{Error: e.Value.Error})
			}
		}
	}
	args := struct {
		ServerName  string
		CommandLine string
		Vtrace      *Tracer
		Pattern     string
		Entries     []entry
	}{
		ServerName:  server,
		CommandLine: fmt.Sprintf("debug glob %q", pattern),
		Pattern:     pattern,
		Vtrace:      tracer,
		Entries:     entries,
	}
	h.execute(h.ctx, w, r, "glob.html", args)
}

type profilesHandler struct{ *handler }

func (h *profilesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		server = r.FormValue("n")
		name   = naming.Join(server, "__debug", "pprof")
	)
	if len(server) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Must specify a server with the URL query parameter 'n'")
		return
	}
	if path := strings.TrimSuffix(r.URL.Path, "/"); strings.HasSuffix(path, strings.TrimSuffix(browseProfilesPath, "/")) {
		urlPrefix := fmt.Sprintf("%s/pprof", strings.TrimPrefix(browseProfilesPath, "/"))
		args := struct {
			ServerName  string
			CommandLine string
			Vtrace      *Tracer
			URLPrefix   string
		}{
			ServerName:  server,
			CommandLine: fmt.Sprintf("debug pprof run %q", name),
			URLPrefix:   urlPrefix,
		}
		h.execute(h.ctx, w, r, "profiles.html", args)
		return
	}
	pproflib.PprofProxy(h.ctx, browseProfilesPath, name).ServeHTTP(w, r)
}

const (
	allTracesLabel       = ">0s"
	oneMsTraceLabel      = ">1ms"
	tenMsTraceLabel      = ">10ms"
	hundredMsTraceLabel  = ">100ms"
	oneSecTraceLabel     = ">1s"
	tenSecTraceLabel     = ">10s"
	hundredSecTraceLabel = ">100s"
)

var (
	labelToCutoff = map[string]float64{
		allTracesLabel:       0,
		oneMsTraceLabel:      0.001,
		tenMsTraceLabel:      0.01,
		hundredMsTraceLabel:  0.1,
		oneSecTraceLabel:     1,
		tenSecTraceLabel:     10,
		hundredSecTraceLabel: 100,
	}
	sortedLabels = []string{allTracesLabel, oneMsTraceLabel, tenMsTraceLabel, hundredMsTraceLabel, oneSecTraceLabel, tenSecTraceLabel, hundredSecTraceLabel}
)

type traceWithStart struct {
	ID    string
	Start time.Time
}

type traceSort []traceWithStart

func (t traceSort) Len() int           { return len(t) }
func (t traceSort) Less(i, j int) bool { return t[i].Start.After(t[j].Start) }
func (t traceSort) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }

func bucketTraces(ctx *context.T, name string) (map[string]traceSort, error) {
	stub := svtrace.StoreClient(name)

	call, err := stub.AllTraces(ctx)
	if err != nil {
		return nil, err
	}
	stream := call.RecvStream()
	var missingTraces []string
	bucketedTraces := map[string]traceSort{}
	for stream.Advance() {
		trace := stream.Value()
		node := vtrace.BuildTree(&trace)
		if node == nil {
			missingTraces = append(missingTraces, trace.Id.String())
			continue
		}
		startTime := findStartTime(node)
		endTime := findEndTime(node)
		traceNode := traceWithStart{ID: trace.Id.String(), Start: startTime}
		duration := endTime.Sub(startTime).Seconds()
		if endTime.IsZero() || startTime.IsZero() {
			// Set the duration so something small but greater than zero so it shows up in
			// the first bucket.
			duration = 0.0001
		}
		for l, cutoff := range labelToCutoff {
			if duration >= cutoff {
				bucketedTraces[l] = append(bucketedTraces[l], traceNode)
			}
		}
	}

	for _, v := range bucketedTraces {
		sort.Sort(v)
	}
	if err := stream.Err(); err != nil {
		return bucketedTraces, err
	}
	if err := call.Finish(); err != nil {
		return bucketedTraces, err
	}
	if len(missingTraces) > 0 {
		return bucketedTraces, fmt.Errorf("missing information for: %s", strings.Join(missingTraces, ","))
	}
	return bucketedTraces, nil
}

type allTracesHandler struct{ *handler }

func (a *allTracesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		server = r.FormValue("n")
		name   = naming.Join(server, "__debug", "vtrace")
	)
	if len(server) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Must specify a server with the URL query parameter 'n'")
		return
	}
	ctx, tracer := newTracer(a.ctx)
	buckets, err := bucketTraces(ctx, name)

	data := struct {
		Buckets      map[string]traceSort
		SortedLabels []string
		Err          error
		ServerName   string
		CommandLine  string
		Vtrace       *Tracer
	}{
		Buckets:      buckets,
		SortedLabels: sortedLabels,
		Err:          err,
		CommandLine:  fmt.Sprintf("debug vtraces %q", name),
		ServerName:   server,
		Vtrace:       tracer,
	}
	a.execute(a.ctx, w, r, "alltrace.html", data)
}

type divTree struct {
	ID string
	// Start is a value from 0-100 which is the percentage of time into the parent span's duration
	// that this span started.
	Start int
	// Width is a value from 0-100 which is the percentage of time in the parent span's duration this span
	// took
	Width       int
	Name        string
	Annotations []annotation
	Children    []divTree
}

type annotation struct {
	// Position is a value from 0-100 which is the percentage of time into the
	// span's duration that this annotation occurred.
	Position int
	Msg      string
}

func convertToTree(n *vtrace.Node, parentStart time.Time, parentEnd time.Time) *divTree {
	// If either start of end is missing, use the parent start/end.
	startTime := n.Span.Start
	if startTime.IsZero() {
		startTime = parentStart
	}

	endTime := n.Span.End
	if endTime.IsZero() {
		endTime = parentEnd
	}

	parentDuration := parentEnd.Sub(parentStart).Seconds()
	start := int(100 * startTime.Sub(parentStart).Seconds() / parentDuration)
	end := int(100 * endTime.Sub(parentStart).Seconds() / parentDuration)
	width := end - start
	if width == 0 {
		width = 1
	}
	top := &divTree{
		ID:    n.Span.Id.String(),
		Start: start,
		Width: width,
		Name:  n.Span.Name,
	}

	top.Annotations = make([]annotation, len(n.Span.Annotations))
	for i, a := range n.Span.Annotations {
		top.Annotations[i].Msg = a.Message
		if a.When.IsZero() {
			top.Annotations[i].Position = 0
			continue
		}
		top.Annotations[i].Position = int(100*a.When.Sub(parentStart).Seconds()/parentDuration) - start
	}

	top.Children = make([]divTree, len(n.Children))
	for i, c := range n.Children {
		top.Children[i] = *convertToTree(c, startTime, endTime)
	}
	return top

}

// findStartTime returns the start time of a node.  The start time is defined as either the span start if it exists
// or the timestamp of the first annotation/sub span.
func findStartTime(n *vtrace.Node) time.Time {
	if !n.Span.Start.IsZero() {
		return n.Span.Start
	}
	var startTime time.Time
	for _, a := range n.Span.Annotations {
		startTime = a.When
		if !startTime.IsZero() {
			break
		}
	}
	for _, c := range n.Children {
		childStartTime := findStartTime(c)
		if startTime.IsZero() || (!childStartTime.IsZero() && startTime.After(childStartTime)) {
			startTime = childStartTime
		}
		if !startTime.IsZero() {
			break
		}
	}
	return startTime
}

// findEndTime returns the end time of a node.  The end time is defined as either the span end if it exists
// or the timestamp of the last annotation/sub span.
func findEndTime(n *vtrace.Node) time.Time {
	if !n.Span.End.IsZero() {
		return n.Span.End
	}

	size := len(n.Span.Annotations)
	var endTime time.Time
	for i := range n.Span.Annotations {
		endTime = n.Span.Annotations[size-1-i].When
		if !endTime.IsZero() {
			break
		}
	}

	size = len(n.Children)
	for i := range n.Children {
		childEndTime := findEndTime(n.Children[size-1-i])
		if endTime.IsZero() || (!childEndTime.IsZero() && childEndTime.After(endTime)) {
			endTime = childEndTime
		}
		if !endTime.IsZero() {
			break
		}
	}
	return endTime
}

type vtraceHandler struct{ *handler }

func (v *vtraceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		server  = r.FormValue("n")
		traceID = r.FormValue("t")
		name    = naming.Join(server, "__debug", "vtrace")
	)

	if len(server) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Must specify a server with the URL query parameter 'n'")
		return
	}

	stub := svtrace.StoreClient(name)
	ctx, tracer := newTracer(v.ctx)
	id, err := uniqueid.FromHexString(traceID)

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Invalid trace id %s: %v", traceID, err)
		return
	}

	trace, err := stub.Trace(ctx, id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Unknown trace id: %s", traceID)
		return
	}

	var buf bytes.Buffer

	vtrace.FormatTrace(&buf, &trace, nil)
	node := vtrace.BuildTree(&trace)

	if node == nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "coud not find root span for trace")
		return
	}

	tree := convertToTree(node, findStartTime(node), findEndTime(node))
	data := struct {
		ID          string
		Root        *divTree
		ServerName  string
		CommandLine string
		DebugTrace  string
		Vtrace      *Tracer
	}{
		ID:          traceID,
		Root:        tree,
		ServerName:  server,
		CommandLine: fmt.Sprintf("debug vtraces %q", name),
		Vtrace:      tracer,
		DebugTrace:  buf.String(),
	}
	v.execute(v.ctx, w, r, "vtrace.html", data)
}

//nolint:deadcode,unused
func internalServerError(w http.ResponseWriter, doing string, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintf(w, "Problem %s: %v", doing, err)
}

//nolint:deadcode,unused
func badRequest(w http.ResponseWriter, problem string) {
	w.WriteHeader(http.StatusBadRequest)
	fmt.Fprint(w, problem)
}

func writeEvent(w http.ResponseWriter, data string) {
	fmt.Fprintf(w, "data: %s\n\n", strings.TrimSpace(data))
}

func writeErrorEvent(w http.ResponseWriter, err error) {
	id := fmt.Sprintf("%v", verror.ErrorID(err))
	writeEvent(w, fmt.Sprintf("ERROR(%v): %v", html.EscapeString(id), html.EscapeString(err.Error())))
}

// Tracer forces collection of a trace rooted at the call to newTracer.
type Tracer struct {
	ctx  *context.T
	span vtrace.Span
}

func newTracer(ctx *context.T) (*context.T, *Tracer) {
	ctx, span := vtrace.WithNewTrace(ctx)
	vtrace.ForceCollect(ctx, 0)
	return ctx, &Tracer{ctx, span}
}

func (t *Tracer) String() string {
	if t == nil {
		return ""
	}
	tr := vtrace.GetStore(t.ctx).TraceRecord(t.span.Trace())
	if len(tr.Spans) == 0 {
		// Do not bother with empty traces
		return ""
	}
	var buf bytes.Buffer
	// nil as the time.Location is fine because the HTTP "server" time is
	// the same as that of the "client" (typically a browser on localhost).
	vtrace.FormatTrace(&buf, vtrace.GetStore(t.ctx).TraceRecord(t.span.Trace()), nil)
	return buf.String()
}

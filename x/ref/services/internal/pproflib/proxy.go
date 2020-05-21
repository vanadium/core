// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package pproflib defines a client-side proxy and server-side implementation
// of the v.io/v23/services/pprof interface.
//
// It is functionally equivalent to http://golang.org/pkg/net/http/pprof/,
// except that the data comes from a remote vanadium server, and the handlers
// are not registered in DefaultServeMux.
package pproflib

import (
	"bufio"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"

	"v.io/v23/context"
	"v.io/v23/services/pprof"
	"v.io/v23/vtrace"
)

// PprofProxy returns an http.Handler implements to serve profile information
// of a remote process with the vanadium object name 'name'.
//
// The handler assumes that it is serving paths under "pathPrefix".
func PprofProxy(ctx *context.T, pathPrefix, name string) http.Handler {
	return &proxy{
		ctx:        ctx,
		name:       name,
		pathPrefix: pathPrefix,
	}
}

type proxy struct {
	ctx        *context.T
	name       string
	pathPrefix string
}

func (p *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch path := p.canonicalPath(r); path {
	case "/":
		http.Redirect(w, r, r.URL.Path+"/pprof/", http.StatusTemporaryRedirect)
	case "/pprof/cmdline":
		p.cmdLine(w, r)
	case "/pprof/profile":
		p.profile(w, r)
	case "/pprof/symbol":
		p.symbol(w, r)
	default:
		if strings.HasPrefix(path, "/pprof/") || path == "/pprof" {
			p.index(w, r)
		} else {
			http.NotFound(w, r)
		}
	}
}

func (p *proxy) canonicalPath(r *http.Request) string {
	return strings.TrimPrefix(r.URL.Path, p.pathPrefix)
}

func replyUnavailable(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusServiceUnavailable)
	fmt.Fprintf(w, "%v", err)
}

// index serves an html page for pprof/ and, indirectly, raw data for pprof/*
func (p *proxy) index(w http.ResponseWriter, r *http.Request) {
	if path := p.canonicalPath(r); strings.HasPrefix(path, "/pprof/") {
		name := strings.TrimPrefix(path, "/pprof/")
		if name != "" {
			p.sendProfile(name, w, r)
			return
		}
	}
	c := pprof.PProfClient(p.name)
	ctx, _ := vtrace.WithNewTrace(p.ctx)
	profiles, err := c.Profiles(ctx)
	if err != nil {
		replyUnavailable(w, err)
		return
	}
	if err := indexTmpl.Execute(w, profiles); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "%v", err)
		return
	}
}

// sendProfile sends the requested profile as a response.
func (p *proxy) sendProfile(name string, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	debug, _ := strconv.Atoi(r.FormValue("debug"))
	c := pprof.PProfClient(p.name)
	ctx, _ := vtrace.WithNewTrace(p.ctx)
	prof, err := c.Profile(ctx, name, int32(debug))
	if err != nil {
		replyUnavailable(w, err)
		return
	}
	iterator := prof.RecvStream()
	for iterator.Advance() {
		_, err := w.Write(iterator.Value())
		if err != nil {
			return
		}
	}
	if err := iterator.Err(); err != nil {
		replyUnavailable(w, err)
		return
	}
	if err := prof.Finish(); err != nil {
		replyUnavailable(w, err)
		return
	}
}

// profile replies with a CPU profile.
func (p *proxy) profile(w http.ResponseWriter, r *http.Request) {
	sec, _ := strconv.ParseUint(r.FormValue("seconds"), 10, 64)
	if sec == 0 {
		sec = 30
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	c := pprof.PProfClient(p.name)
	ctx, _ := vtrace.WithNewTrace(p.ctx)
	prof, err := c.CpuProfile(ctx, int32(sec))
	if err != nil {
		replyUnavailable(w, err)
		return
	}
	iterator := prof.RecvStream()
	for iterator.Advance() {
		_, err := w.Write(iterator.Value())
		if err != nil {
			return
		}
	}
	if err := iterator.Err(); err != nil {
		replyUnavailable(w, err)
		return
	}
	if err := prof.Finish(); err != nil {
		replyUnavailable(w, err)
		return
	}
}

// cmdLine replies with the command-line arguments of the process.
func (p *proxy) cmdLine(w http.ResponseWriter, r *http.Request) {
	c := pprof.PProfClient(p.name)
	ctx, _ := vtrace.WithNewTrace(p.ctx)
	cmdline, err := c.CmdLine(ctx)
	if err != nil {
		replyUnavailable(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, strings.Join(cmdline, "\x00"))
}

// symbol replies with a map of program counters to object name.
func (p *proxy) symbol(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	var b *bufio.Reader
	if r.Method == "POST" {
		b = bufio.NewReader(r.Body)
	} else {
		b = bufio.NewReader(strings.NewReader(r.URL.RawQuery))
	}
	pcList := []uint64{}
	for {
		word, err := b.ReadSlice('+')
		if err == nil {
			word = word[0 : len(word)-1] // trim +
		}
		pc, _ := strconv.ParseUint(string(word), 0, 64)
		if pc != 0 {
			pcList = append(pcList, pc)
		}
		if err != nil {
			if err != io.EOF {
				replyUnavailable(w, err)
				return
			}
			break
		}
	}
	c := pprof.PProfClient(p.name)
	ctx, _ := vtrace.WithNewTrace(p.ctx)
	pcMap, err := c.Symbol(ctx, pcList)
	if err != nil {
		replyUnavailable(w, err)
		return
	}
	if len(pcMap) != len(pcList) {
		replyUnavailable(w, fmt.Errorf("received the wrong number of results. Got %d, want %d", len(pcMap), len(pcList)))
		return
	}
	// The pprof tool always wants num_symbols to be non-zero, even if no
	// symbols are returned.. The actual value doesn't matter.
	fmt.Fprintf(w, "num_symbols: 1\n")
	for i, v := range pcMap {
		if len(v) > 0 {
			fmt.Fprintf(w, "%#x %s\n", pcList[i], v)
		}
	}
}

var indexTmpl = template.Must(template.New("index").Parse(`<html>
<head>
<title>/pprof/</title>
</head>
/pprof/<br>
<br>
<body>
profiles:<br>
<table>
{{range .}}<tr><td align=right><td><a href="/pprof/{{.}}?debug=1">{{.}}</a>
{{end}}
</table>
<br>
<a href="/pprof/goroutine?debug=2">full goroutine stack dump</a><br>
</body>
</html>
`))

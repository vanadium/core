// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"v.io/x/ref/services/ben"
)

// NewHTTPHandler returns a handler that provides web interface for browsing
// benchmark results in store.
func NewHTTPHandler(assets *Assets, store Store) http.Handler {
	return &handler{assets, store}
}

type handler struct {
	assets *Assets
	store  Store
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if file := path.Base(r.URL.Path); strings.HasSuffix(file, ".css") || strings.HasSuffix(file, ".js") {
		data, err := h.assets.File(file)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintln(w, "ERROR:", err)
			return
		}
		if strings.HasSuffix(file, ".css") {
			w.Header().Set("Content-Type", "text/css")
		} else if strings.HasSuffix(file, ".js") {
			w.Header().Set("Content-Type", "application/javascript")
		}
		w.Write(data) //nolint:errcheck
		return
	}
	if id := strings.TrimSpace(r.FormValue("id")); len(id) > 0 {
		bm, itr := h.store.Runs(id)
		defer itr.Close()
		h.runs(w, bm, itr)
		return
	}
	if qstr := strings.TrimSpace(r.FormValue("q")); len(qstr) > 0 {
		query, err := ParseQuery(qstr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			args := struct {
				Query string
				Error error
			}{qstr, err}
			h.executeTemplate(w, tmplBadQuery, args)
			return
		}
		h.handleQuery(w, query)
		return
	}
	if src := strings.TrimSpace(r.FormValue("s")); len(src) > 0 {
		h.describeSource(w, src)
		return
	}
	h.executeTemplate(w, tmplHome, nil)
}

func (h *handler) handleQuery(w http.ResponseWriter, query *Query) {
	bmarks := h.store.Benchmarks(query)
	defer bmarks.Close()
	if !bmarks.Advance() {
		h.executeTemplate(w, tmplNoBenchmarks, query)
		return
	}
	bm := bmarks.Value()
	// Advance once more, if there are more benchmarks (different names,
	// different scenarios, different uploaders) for the same query, then
	// present a list to choose from.
	if !bmarks.Advance() {
		itr := bmarks.Runs()
		defer itr.Close()
		h.runs(w, bm, itr)
		return
	}
	h.benchmarks(w, query, bm, bmarks)
}

func (h *handler) benchmarks(w http.ResponseWriter, query *Query, first Benchmark, itr BenchmarkIterator) {
	var (
		cancel = make(chan struct{})
		items  = make(chan Benchmark, 2)
		errs   = make(chan error, 1)
	)
	defer close(cancel)
	items <- first
	items <- itr.Value()
	go func() {
		defer close(errs)
		defer close(items)
		for itr.Advance() {
			select {
			case items <- itr.Value():
			case <-cancel:
				return
			}
			if err := itr.Err(); err != nil {
				errs <- err
			}
		}
	}()
	args := struct {
		Query *Query
		Items <-chan Benchmark
		Err   <-chan error
	}{
		Query: query,
		Items: items,
		Err:   errs,
	}
	h.executeTemplate(w, tmplBenchmarks, args)
}

func (h *handler) runs(w http.ResponseWriter, bm Benchmark, itr RunIterator) {
	type item struct {
		Run          ben.Run
		SourceCodeID string
		UploadTime   time.Time
		Index        int
	}
	var (
		cancel = make(chan struct{})
		items  = make(chan item)
		errs   = make(chan error, 1)
	)
	defer close(cancel)
	go func() {
		defer close(errs)
		defer close(items)
		idx := 0
		for itr.Advance() {
			idx++
			r, s, t := itr.Value()
			select {
			case items <- item{r, s, t, idx}:
			case <-cancel:
				return
			}
		}
		if err := itr.Err(); err != nil {
			errs <- err
		}
	}()
	args := struct {
		Benchmark Benchmark
		Items     <-chan item
		Err       <-chan error
	}{
		Benchmark: bm,
		Items:     items,
		Err:       errs,
	}
	h.executeTemplate(w, tmplRuns, args)
}

func (h *handler) describeSource(w http.ResponseWriter, src string) {
	w.Header().Set("Content-Type", "text/plain")
	code, err := h.store.DescribeSource(src)
	if err != nil {
		fmt.Fprintf(w, "ERROR:%v", err)
		return
	}
	fmt.Fprintln(w, code)
}

func (h *handler) executeTemplate(w http.ResponseWriter, tmpl string, args interface{}) {
	t, err := h.assets.Template(tmpl)
	if err != nil {
		fmt.Fprintln(w, "ERROR:Failed to create template:", err)
		return
	}
	if err := t.Execute(w, args); err != nil {
		fmt.Fprintf(w, "ERROR:%v", err)
	}

}

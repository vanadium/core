// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate ./gen_assets.sh
package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"v.io/v23/context"
	"v.io/x/ref/services/allocator/allocatord/assets"
)

type assetsHelper struct {
	dir string
	// Pre-parsed form of the compiled-in templates.
	// Non-nil iff dir is empty.
	cache map[string]*template.Template
}

var contentTypes = map[string]string{
	".css": "text/css",
	".js":  "application/javascript",
}

const (
	badRequestTmpl    = "bad-request.tmpl.html"
	dashboardTmpl     = "dashboard.tmpl.html"
	errorTmpl         = "error.tmpl.html"
	homeTmpl          = "home.tmpl.html"
	rootTmpl          = "root.tmpl.html"
	headPartialTmpl   = "head.tmpl.html"
	headerPartialTmpl = "header.tmpl.html"
)

// newAssetsHelper returns an object that provides compiled-in assets if dir is
// empty or reads them from dir otherwise.
//
// A non-empty dir is typically provided when iterating on the contents of the
// assets before release.
func newAssetsHelper(dir string) (*assetsHelper, error) {
	a := &assetsHelper{dir: dir}
	if dir != "" {
		return a, nil
	}
	// Validate the compiled-in templates and cache them.
	cache := make(map[string]*template.Template)
	for _, n := range assets.AssetNames() {
		if strings.HasSuffix(n, ".tmpl.html") {
			t, err := a.template(n)
			if err != nil {
				return nil, err
			}
			cache[n] = t
		}
	}
	a.cache = cache
	return a, nil
}

func (a *assetsHelper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	file := path.Base(r.URL.Path)
	ext := filepath.Ext(file)
	contentType, ok := contentTypes[ext]
	if !ok {
		http.Error(w, fmt.Sprintf("file type %q not supported", ext), http.StatusInternalServerError)
		return
	}
	data, err := a.file(file)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Write(data)
	return
}

func (a *assetsHelper) file(name string) ([]byte, error) {
	if a.dir == "" {
		data, err := assets.Asset(name)
		if err != nil {
			return nil, fmt.Errorf("failed to load compiled-in asset %q: %v", name, err)
		}
		return data, nil
	}
	return ioutil.ReadFile(filepath.Join(a.dir, name))
}

func (a *assetsHelper) template(name string) (*template.Template, error) {
	if a.cache != nil {
		if t := a.cache[name]; t != nil {
			return t, nil
		}
		return nil, fmt.Errorf("template %q not compiled into binary", name)
	}
	data, err := a.file(name)
	if err != nil {
		return nil, err
	}
	t, err := template.New(name).Funcs(template.FuncMap{
		"title": strings.Title,
	}).Parse(string(data))
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (a *assetsHelper) executeTemplate(w http.ResponseWriter, tmpl string, args interface{}) error {
	t, err := a.template(tmpl)
	if err != nil {
		return err
	}
	addPartial := func(partialTmpl string) error {
		partialData, err := a.file(partialTmpl)
		if err != nil {
			return err
		}
		t, err = t.Parse(string(partialData))
		return err
	}
	if err := addPartial(headPartialTmpl); err != nil {
		return err
	}
	if err := addPartial(headerPartialTmpl); err != nil {
		return err
	}
	return t.Execute(w, args)
}

func (a *assetsHelper) errorOccurred(ctx *context.T, w http.ResponseWriter, r *http.Request, homeURL string, err error) {
	tmplArgs := struct {
		URL, Home string
		Error     error
	}{
		URL:   r.URL.String(),
		Home:  homeURL,
		Error: err,
	}
	if err := a.executeTemplate(w, errorTmpl, tmplArgs); err != nil {
		ctx.Errorf("Failed to execute template: %v", err)
	}
}

func (a *assetsHelper) badRequest(ctx *context.T, w http.ResponseWriter, r *http.Request, err error) {
	tmplArgs := struct {
		Path  string
		Error error
	}{
		Path:  r.URL.Path,
		Error: err,
	}
	if err := a.executeTemplate(w, badRequestTmpl, tmplArgs); err != nil {
		ctx.Errorf("Failed to execute template: %v", err)
	}
}

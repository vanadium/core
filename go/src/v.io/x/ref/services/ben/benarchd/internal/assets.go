// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate ./gen_assets.sh
package internal

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strings"

	"v.io/x/ref/services/ben/benarchd/internal/assets"
)

// NewAssets returns an object that provides compiled-in assets if dir is empty
// or reads them from dir otherwise.
//
// A non-empty dir is typically provided when iterating on the contents of the
// assets before release.
func NewAssets(dir string) (*Assets, error) {
	a := &Assets{dir: dir}
	if dir != "" {
		return a, nil
	}
	// Validate the compiled-in templates and cache them.
	cache := make(map[string]*template.Template)
	for _, n := range assets.AssetNames() {
		if strings.HasSuffix(n, ".tmpl.html") && n != "styling.tmpl.html" && n != "footer.tmpl.html" {
			t, err := a.Template(n)
			if err != nil {
				return nil, err
			}
			cache[n] = t
		}
	}
	a.cache = cache
	return a, nil
}

const (
	tmplHome         = "home.tmpl.html"
	tmplBadQuery     = "badquery.tmpl.html"
	tmplNoBenchmarks = "nobenchmarks.tmpl.html"
	tmplBenchmarks   = "benchmarks.tmpl.html"
	tmplRuns         = "runs.tmpl.html"
)

type Assets struct {
	dir string
	// Pre-parsed form of the compiled-in templates.
	// Non-nil iff dir is empty.
	cache map[string]*template.Template
}

func (a *Assets) File(name string) ([]byte, error) {
	if a.dir == "" {
		data, err := assets.Asset(name)
		if err != nil {
			return nil, fmt.Errorf("failed to load compiled-in asset %q: %v", name, err)
		}
		return data, nil
	}
	return ioutil.ReadFile(filepath.Join(a.dir, name))
}

func (a *Assets) Template(name string) (*template.Template, error) {
	if a.cache != nil {
		if t := a.cache[name]; t != nil {
			return t, nil
		}
		return nil, fmt.Errorf("template %q not compiled into binary", name)
	}
	t := template.New(name)
	if name == tmplBenchmarks {
		t = t.Funcs(template.FuncMap{
			"refineQuery": func(q *Query, field, value string) (*Query, error) {
				ret := *q
				f := reflect.ValueOf(&ret).Elem().FieldByName(field)
				var invalid reflect.Value
				if f == invalid {
					return nil, fmt.Errorf("%q is not a valid query field", field)
				}
				f.Set(reflect.ValueOf(value))
				return &ret, nil
			},
		})
	}
	files := []string{name, "styling.tmpl.html", "footer.tmpl.html"}
	for _, f := range files {
		data, err := a.File(f)
		if err != nil {
			return nil, err
		}
		if t, err = t.Parse(string(data)); err != nil {
			return nil, err
		}
	}
	return t, nil
}

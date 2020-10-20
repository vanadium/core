// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package verror

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
)

type pathCache struct {
	sync.Mutex
	paths map[string]string
}

func enclosingGoMod(dir string) (string, error) {
	for {
		gomodfile := filepath.Join(dir, "go.mod")
		if fi, err := os.Stat(gomodfile); err == nil && !fi.IsDir() {
			return dir, nil
		}
		d := filepath.Dir(dir)
		if d == dir {
			return "", fmt.Errorf("failed to find enclosing go.mod for dir %v", dir)
		}
		dir = d
	}
}

var pkgPathCache = pathCache{
	paths: make(map[string]string),
}

func (pc *pathCache) has(dir string) (string, bool) {
	pc.Lock()
	defer pc.Unlock()
	p, ok := pc.paths[dir]
	return p, ok
}

func (pc *pathCache) set(dir, pkg string) {
	pc.Lock()
	defer pc.Unlock()
	pc.paths[dir] = pkg
}

// IDPath returns a string of the form <package-path>.<name>
// where <package-path> is derived from the type of the supplied
// value. Typical usage would be except that dummy can be replaced
// by an existing type defined in the package.
//
//  type dummy int
//  verror.ID(verror.IDPath(dummy(0), "MyError"))
//
func IDPath(val interface{}, id string) ID {
	return ID(reflect.TypeOf(val).PkgPath() + "." + id)
}

/*
func longestCommonSuffix(pkgPath, filename string) string {
	longest := ""
	for {
		fl := filepath.Base(filename)
		pl := path.Base(pkgPath)
		if fl == pl {
			longest = path.Join(fl, longest)
			filename = filepath.Dir(filename)
			pkgPath = path.Dir(pkgPath)
			continue
		}
		break
	}
	return longest
}

var thisPkg string
var thisPkgOnce sync.Once

func initThisPkg() {
	type dummy int
	thisPkg = reflect.TypeOf(dummy(0)).PkgPath()
}
*/

var thisDir string

func init() {
	_, file, _, _ := runtime.Caller(0)
	thisDir := filepath.Dir(file)
}

func (pc *pathCache) pkgPath(file string) string {
	thisPkgOnce.Do(initThisPkg)
	pkgPath := longestCommonSuffix(thisPkg, filepath.Dir(file))
	if len(pkgPath) == 0 {
		return ""
	}
	pkgPath = path.Join(strings.TrimSuffix(thisPkg, pkgPath), pkgPath)
	pc.set(filepath.Dir(file), pkgPath)
	return pkgPath
}

func ensurePackagePath(id ID) ID {
	fmt.Printf("EP: %v\n", id)
	sid := string(id)
	if strings.Contains(sid, ".") && sid[0] != '.' {
		fmt.Printf("EP: 1 %v\n", id)

		return id
	}
	_, file, _, _ := runtime.Caller(2)
	pkg := pkgPathCache.pkgPath(file)
	if len(pkg) == 0 {
		fmt.Printf("EP: 2 %v ... %v\n", file, id)
		return id
	}
	if strings.HasPrefix(sid, pkg) {
		fmt.Printf("EP: 3 %v\n", id)

		return id
	}
	if strings.HasPrefix(sid, ".") {
		fmt.Printf("EP: 4 %v\n", id)

		return ID(pkg + sid)
	}
	fmt.Printf("SEP: %v -> %v\n", id, ID(pkg+"."+sid))
	return ID(pkg + "." + sid)
}

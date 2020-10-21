// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package verror

import (
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

// longestCommonSuffix for a package path and filename.
func longestCommonSuffix(pkgPath, filename string) (string, string) {
	longestPkg, longestFilePath := "", ""
	for {
		fl := filepath.Base(filename)
		pl := path.Base(pkgPath)
		if fl == pl {
			longestPkg = path.Join(fl, longestPkg)
			longestFilePath = filepath.Join(fl, longestFilePath)
			filename = filepath.Dir(filename)
			pkgPath = path.Dir(pkgPath)
			if fl == "/" {
				break
			}
			continue
		}
		break
	}
	return longestPkg, longestFilePath
}

type pathState struct {
	pkg string // pkg path for the value passed to init
	dir string // the directory component for the file passed to init
	// The portion of the local file path that is outside of the go module,
	// e.g. for /a/b/c/core/v23/verror it would be /a/b/c/core.
	filePrefix string
	// the portion of the package path that does not appear in the file name,
	// e.g. for /a/b/c/core/v23/verror and v.io/v23/verror it would be v.io.
	pkgPrefix string
}

func (ps *pathState) init(pkgPath string, file string) {
	ps.pkg = pkgPath
	ps.dir = filepath.Dir(file)
	pkgLCS, fileLCS := longestCommonSuffix(ps.pkg, ps.dir)
	ps.filePrefix = filepath.Clean(strings.TrimSuffix(ps.dir, fileLCS))
	ps.pkgPrefix = path.Clean(strings.TrimSuffix(ps.pkg, pkgLCS))
}

var (
	ps       = &pathState{}
	initOnce sync.Once
)

func convertFileToPkgName(filename string) string {
	return path.Clean(strings.ReplaceAll(filename, string(filepath.Separator), "/"))
}

func (pc *pathCache) pkgPath(file string) string {
	initOnce.Do(func() {
		type dummy int
		_, file, _, _ := runtime.Caller(0)
		ps.init(reflect.TypeOf(dummy(0)).PkgPath(), file)
	})
	pdir := filepath.Dir(file)
	rel := strings.TrimPrefix(pdir, ps.filePrefix)
	if rel == pdir {
		return ""
	}
	relPkg := convertFileToPkgName(rel)
	pkgPath := path.Join(ps.pkgPrefix, relPkg)
	pc.set(filepath.Dir(file), pkgPath)
	return pkgPath
}

func ensurePackagePath(id ID) ID {
	sid := string(id)
	if strings.Contains(sid, ".") && sid[0] != '.' {
		return id
	}
	_, file, _, _ := runtime.Caller(2)
	pkg := pkgPathCache.pkgPath(file)
	if len(pkg) == 0 {
		return id
	}
	if strings.HasPrefix(sid, pkg) {
		return id
	}
	if strings.HasPrefix(sid, ".") {
		return ID(pkg + sid)
	}
	return ID(pkg + "." + sid)
}

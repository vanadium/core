package verror

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/mod/modfile"
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

func (pc *pathCache) pkgPath(file string) (string, error) {
	dir := filepath.Clean(filepath.Dir(file))
	if p, ok := pc.has(dir); ok {
		return p, nil
	}
	root, err := enclosingGoMod(dir)
	if err != nil {
		return "", err
	}
	gomodfile := filepath.Join(root, "go.mod")
	gomod, err := ioutil.ReadFile(gomodfile)
	if err != nil {
		return "", err
	}
	module := modfile.ModulePath(gomod)
	if len(module) == 0 {
		return "", fmt.Errorf("failed to read module path from %v", gomodfile)
	}

	pkgPath := strings.TrimPrefix(dir, root)
	if !strings.HasPrefix(pkgPath, module) {
		pkgPath = path.Join(module, pkgPath)
	}
	pc.set(dir, pkgPath)
	return pkgPath, nil
}

func ensurePackagePath(id ID) ID {
	_, file, _, _ := runtime.Caller(2)
	pkg, err := pkgPathCache.pkgPath(file)
	if err != nil {
		panic(fmt.Sprintf("failed to determine package name for %v: %v", file, err))
	}
	sid := string(id)
	if strings.HasPrefix(sid, pkg) {
		return id
	}
	if strings.HasPrefix(sid, ".") {
		return ID(pkg + sid)
	}
	return ID(pkg + "." + sid)
}

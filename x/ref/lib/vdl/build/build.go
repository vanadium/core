// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package build implements utilities to collect VDL build information and run
// the parser and compiler.
//
// VDL Packages
//
// VDL is organized into packages, where a package is a collection of one or
// more source files.  The files in a package collectively define the types,
// constants, services and errors belonging to the package; these are called
// package elements.
//
// The package elements in package P may be used in another package Q.  First
// package Q must import package P, and then refer to the package elements in P.
// Imports define the package dependency graph, which must be acyclic.
//
// Build Strategy
//
// The steps to building a VDL package P:
//   1) Compute the transitive closure of P's dependencies DEPS.
//   2) Sort DEPS in dependency order.
//   3) Build each package D in DEPS.
//   3) Build package P.
//
// Building a package P requires that all elements used by P are understood,
// including elements defined outside of P.  The only way for a change to
// package Q to affect the build of P is if Q is in the transitive closure of
// P's package dependencies.  However there may be false positives; the change
// to Q might not actually affect P.
//
// The build process may perform more work than is strictly necessary, because
// of these false positives.  However it is simple and correct.
//
// The TransitivePackages* functions implement build steps 1 and 2.
//
// The Build* functions implement build steps 3 and 4.
//
// Other functions provide related build information and utilities.
package build

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
	"v.io/v23/vdl"
	"v.io/v23/vdlroot/vdltool"
	"v.io/x/lib/toposort"
	"v.io/x/ref/lib/vdl/build/internal/builtinvdlroot"
	"v.io/x/ref/lib/vdl/compile"
	"v.io/x/ref/lib/vdl/parse"
	"v.io/x/ref/lib/vdl/vdlutil"
)

const vdlrootImportPrefix = "v.io/v23/vdlroot"

// Package represents the build information for a vdl package.
type Package struct {
	// Name is the name of the package, specified in the vdl files.
	// E.g. "bar"
	Name string
	// IsRoot is true iff this package is a vdlroot standard package.
	IsRoot bool
	// IsBuiltIn is true iff the data for this package is built-in to the binary,
	// rather than read off files from the filesystem.
	IsBuiltIn bool
	// Path is the package path; the path used in VDL import clauses.
	// E.g. "foo/bar".
	Path string
	// GenPath is the package path to use for code generation.  It is the same as
	// Path, except for vdlroot standard packages.
	// E.g. "v.io/v23/vdlroot/time"
	GenPath string
	// Dir is the absolute directory containing the package files.
	// E.g. "/home/user/vanadium/vdl/src/foo/bar"
	Dir string
	// BaseFileNames is the list of sorted base vdl file names for this package.
	// Join these with Dir to get absolute file names.
	BaseFileNames []string
	// Config is the configuration for this package, specified by an optional
	// "vdl.config" file in the package directory.  If no "vdl.config" file
	// exists, the zero value of Config is used.
	Config vdltool.Config

	// OpenFilesFunc is a function that opens the files with the given fileNames,
	// and returns a map from base file name to file contents.
	OpenFilesFunc func(fileNames []string) (map[string]io.ReadCloser, error)

	// filesToClose holds files that need to be closed.
	filesToClose []io.Closer
}

// UnknownPathMode specifies the behavior when an unknown path is encountered.
type UnknownPathMode int

const (
	UnknownPathIsIgnored UnknownPathMode = iota // Silently ignore unknown paths
	UnknownPathIsError                          // Produce error for unknown paths
)

func (m UnknownPathMode) String() string {
	switch m {
	case UnknownPathIsIgnored:
		return "UnknownPathIsIgnored"
	case UnknownPathIsError:
		return "UnknownPathIsError"
	default:
		return fmt.Sprintf("UnknownPathMode(%d)", m)
	}
}

func (m UnknownPathMode) logOrErrorf(errs *vdlutil.Errors, format string, v ...interface{}) {
	if m == UnknownPathIsIgnored {
		vdlutil.Vlog.Printf(format, v...)
	} else {
		errs.Errorf(format, v...)
	}
}

func pathPrefixDotOrDotDot(path string) bool {
	// The point of this helper is to catch cases where the path starts with a
	// . or .. element; note that  ... returns false.
	spath := filepath.ToSlash(path)
	return path == "." || path == ".." || strings.HasPrefix(spath, "./") || strings.HasPrefix(spath, "../")
}

func ignorePathElem(elem string) bool {
	return (strings.HasPrefix(elem, ".") && !pathPrefixDotOrDotDot(elem)) ||
		strings.HasPrefix(elem, "_")
}

// validPackagePath returns true iff the path is valid; i.e. if none of the path
// elems is ignored.
func validPackagePath(path string) bool {
	for _, elem := range strings.Split(path, "/") {
		if ignorePathElem(elem) {
			return false
		}
	}
	return true
}

// validFile returns true iff the base file name is a valid vdl file.
func validFile(base string) bool {
	return !ignorePathElem(base) && filepath.Ext(base) == ".vdl"
}

// New packages always start with an empty Name, which is filled in with the
// result of parse.InferPackageName.
func newPackage(path, genPath, dir string, mode UnknownPathMode, opts Opts, vdlenv *compile.Env) *Package {
	pkg := &Package{
		IsRoot:        path != genPath,
		Path:          path,
		GenPath:       genPath,
		Dir:           dir,
		OpenFilesFunc: openFiles,
	}
	if err := pkg.initBaseFileNames(); err != nil {
		mode.logOrErrorf(vdlenv.Errors, "%s: bad package dir (%v)", pkg.Dir, err)
		return nil
	}
	// TODO(toddw): Add a mechanism in vdlutil.Errors to distinguish categories of
	// errors, so that it's more obvious when errors are coming from vdl.config
	// files vs *.vdl files.
	origErrors := vdlenv.Errors.NumErrors()
	if pkg.findAndInitVDLConfig(opts, vdlenv); origErrors != vdlenv.Errors.NumErrors() {
		return nil
	}
	return pkg
}

// initBaseFileNames initializes p.BaseFileNames from the contents of p.Dir.
func (p *Package) initBaseFileNames() error {
	infos, err := ioutil.ReadDir(p.Dir)
	if err != nil {
		return err
	}
	for _, info := range infos {
		if info.IsDir() {
			continue
		}
		base, full := info.Name(), filepath.Join(p.Dir, info.Name())
		if !validFile(base) {
			vdlutil.Vlog.Printf("%s: ignoring file", full)
			continue
		}
		vdlutil.Vlog.Printf("%s: adding vdl file", full)
		p.BaseFileNames = append(p.BaseFileNames, base)
	}
	if len(p.BaseFileNames) == 0 {
		return fmt.Errorf("no vdl files")
	}
	return nil
}

// findAndInitVDLConfig initializes p.Config based on the optional vdl.config file.
func (p *Package) findAndInitVDLConfig(opts Opts, vdlenv *compile.Env) {
	name := path.Join(p.Path, opts.vdlConfigName())
	data, err := os.Open(filepath.Join(p.Dir, opts.vdlConfigName()))
	switch {
	case os.IsNotExist(err):
		return
	case err != nil:
		vdlenv.Errors.Errorf("%s: couldn't open (%v)", name, err)
		return
	}
	p.initVDLConfig(name, data, vdlenv)
	if err := data.Close(); err != nil {
		vdlenv.Errors.Errorf("%s: couldn't close (%v)", name, err)
	}
}

func (p *Package) initVDLConfig(name string, r io.Reader, vdlenv *compile.Env) {
	// Build the vdl.config file with an implicit "vdltool" import.  Note that the
	// actual "vdltool" package should have already been populated into vdlenv.
	if vdlenv.ResolvePackage("vdltool") == nil {
		vdlenv.Errors.Errorf(`%s: package %q cannot specify a config file; it is a transitive dependency of "vdltool"`, name, p.Name)
		return
	}
	BuildConfigValue(name, r, []string{"vdltool"}, vdlenv, &p.Config)
}

// OpenFiles opens all files in the package and returns a map from base file
// name to file contents.  CloseFiles must be called to close the files.
func (p *Package) OpenFiles() (map[string]io.Reader, error) {
	var fileNames []string
	for _, baseName := range p.BaseFileNames {
		fileNames = append(fileNames, filepath.Join(p.Dir, baseName))
	}
	files, err := p.OpenFilesFunc(fileNames)
	if err != nil {
		for _, c := range files {
			c.Close()
		}
		return nil, err
	}
	// Convert map elem type from io.ReadCloser to io.Reader.
	res := make(map[string]io.Reader, len(files))
	for n, f := range files {
		res[n] = f
		p.filesToClose = append(p.filesToClose, f)
	}
	return res, nil
}

func openFiles(fileNames []string) (map[string]io.ReadCloser, error) {
	files := make(map[string]io.ReadCloser, len(fileNames))
	for _, fileName := range fileNames {
		file, err := os.Open(fileName)
		if err != nil {
			for _, c := range files {
				c.Close()
			}
			return nil, err
		}
		files[path.Base(fileName)] = file
	}
	return files, nil
}

func openBuiltInFiles(fileNames []string) (map[string]io.ReadCloser, error) {
	files := make(map[string]io.ReadCloser, len(fileNames))
	for _, fileName := range fileNames {
		file, err := builtinvdlroot.Asset(fileName)
		if err != nil {
			return nil, fmt.Errorf("%s: can't load builtin file: %v", fileName, err)
		}
		files[path.Base(fileName)] = ioutil.NopCloser(bytes.NewReader(file))
	}
	return files, nil
}

// CloseFiles closes all files returned by OpenFiles.  Returns nil if all files
// were closed successfully, otherwise returns one of the errors, dropping the
// others.  Regardless of whether an error is returned, Close will be called on
// all files.
func (p *Package) CloseFiles() error {
	var err error
	for _, c := range p.filesToClose {
		if err2 := c.Close(); err == nil {
			err = err2
		}
	}
	p.filesToClose = nil
	return err
}

// SrcDirs returns a list of package root source directories, based on the
// VDLPATH and VDLROOT environment variables.
//
// VDLPATH is a list of directories separated by filepath.ListSeparator;
// e.g. the separator is ":" on UNIX, and ";" on Windows.  The path below each
// VDLPATH directory determines the vdl import path.
//
// See RootDir for details on VDLROOT.
func SrcDirs(errs *vdlutil.Errors) []string {
	var srcDirs []string
	if root := RootDir(errs); root != "" {
		srcDirs = append(srcDirs, root)
	}
	return append(srcDirs, vdlPathSrcDirs(errs)...)
}

// GoModuleName returns the value of the module statement in the go.mod file in
// the directory specified by path, or an empty string otherwise.
func GoModuleName(path string) (string, error) {
	gomod := filepath.Join(path, "go.mod")
	buf, err := ioutil.ReadFile(gomod)
	if err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
		return "", err
	}
	module := modfile.ModulePath(buf)
	if len(module) == 0 {
		return "", fmt.Errorf("failed to find module statement in %v", gomod)
	}
	return module, nil
}

// PackagePathSplit returns the longest common suffix in dir and path,
// the prefix in dir after the common suffix is removed and the portion of the
// of the original path that is not part of the common prefix. This is useful
// for working with the directory structure commonly used by go modules.
// For example:
//
// PackagePathSplit("a/b/c", "/b/c") yields "a", "", "b/c"
// PackagePathSplit("a/b/c", "m/b/c") yields "a", "m", "b/c"
//
// It is intended to work on Windows, as in:
//
// PackagePathSplit(`a\b\c`, "m/b/c") yields "a", "m", "b/c"
func PackagePathSplit(dir, pkgPath string) (prefix string, body string, suffix string) {
	// Normalize dir path to use / instead of possibly using.
	ndir := strings.ReplaceAll(dir, filePathSeparator, packagePathSeparator)
	suffix = pkgPath
	for {
		idx := strings.Index(suffix, packagePathSeparator)
		if idx < 0 {
			break
		}
		if strings.HasSuffix(ndir, suffix) {
			break
		}
		suffix = suffix[idx+1:]
	}
	prefix = strings.TrimSuffix(ndir, suffix)
	body = strings.TrimSuffix(strings.TrimSuffix(pkgPath, suffix), packagePathSeparator)
	prefix = strings.ReplaceAll(prefix, packagePathSeparator, filePathSeparator)
	prefix = strings.TrimSuffix(prefix, filePathSeparator)
	return
}

func goModules(errs *vdlutil.Errors) map[string]string {
	modPrefix := map[string]string{}
	for _, srcDir := range SrcDirs(errs) {
		goMod, err := GoModuleName(srcDir)
		if err != nil {
			errs.Errorf("failed to read go.mod: %v", err)
			continue
		}
		if len(goMod) > 0 {
			modPrefix[srcDir] = goMod
		}
	}
	return modPrefix
}

// RootDir returns the VDL root directory, based on the VDLROOT environment
// variable.
//
// VDLROOT is a single directory specifying the location of the standard vdl
// packages.  It has the same requirements as VDLPATH components.  The returned
// string may be empty, if VDLROOT is empty or unset.
func RootDir(errs *vdlutil.Errors) string {
	vdlroot := os.Getenv("VDLROOT")
	if vdlroot == "" {
		return ""
	}
	abs, err := filepath.Abs(vdlroot)
	if err != nil {
		errs.Errorf("VDLROOT %q can't be made absolute: %v", vdlroot, err)
		return ""
	}
	if _, err := os.Stat(abs); err != nil {
		errs.Errorf("VDLROOT %q doesn't exist: %v", vdlroot, err)
		return ""
	}
	return abs
}

func vdlPathSrcDirs(errs *vdlutil.Errors) []string {
	var srcDirs []string
	for _, dir := range filepath.SplitList(os.Getenv("VDLPATH")) {
		if dir != "" {
			abs, err := filepath.Abs(dir)
			if err != nil {
				errs.Errorf("VDLPATH src dir %q can't be made absolute (%v)", dir, err)
				continue // keep going to collect all errors
			}
			srcDirs = append(srcDirs, abs)
		}
	}
	if len(srcDirs) == 0 {
		errs.Error("No src dirs; set VDLPATH to a valid value")
		return nil
	}
	return srcDirs
}

// IsDirPath returns true iff the path is absolute, or begins with a . or
// .. element.  The path denotes the package in that directory.
func IsDirPath(path string) bool {
	return filepath.IsAbs(path) || pathPrefixDotOrDotDot(path)
}

// IsImportPath returns true iff !IsDirPath.  The path P denotes the package in
// directory DIR/src/P, for some DIR listed in SrcDirs.
func IsImportPath(path string) bool {
	return !IsDirPath(path)
}

// depSorter does the main work of collecting and sorting packages and their
// dependencies.  The full syntax from the go cmdline tool is supported; we
// allow both dirs and import paths, as well as the "all" and "..." wildcards.
//
// This is slightly complicated because of dirs, and the potential for symlinks.
// E.g. let's say we have two directories, one a symlink to the other:
//   /home/user/go/src/foo/bar/base
//   /home/user/go/src/foo/bar2     symlink to bar
//
// The problem is that if the user has cwd pointing at one of the two "base"
// dirs and specifies a relative directory ".." it's ambiguous which absolute
// dir we'll end up with; file paths form a graph rather than a tree.  For more
// details see http://plan9.bell-labs.com/sys/doc/lexnames.html
//
// This means that if the user builds a dir (rather than an import path), we
// might not be able to deduce the package path.  Note that the error definition
// mechanism relies on the package path to create implicit error ids, and this
// must be known at the time the package is compiled.  To handle this we call
// deducePackagePath and attempt to deduce the package path even if the user
// builds a directory, and return errors if this fails.
//
// TODO(toddw): If we care about performance we could serialize the compiled
// compile.Package information and write it out as compiler-generated artifacts,
// similar to how the regular go tool generates *.a files under the top-level
// pkg directory.
type depSorter struct {
	opts        Opts
	rootDir     string
	srcDirs     []string
	goModules   map[string]string
	builtInRoot map[string]*Package
	pathMap     map[string]*Package
	dirMap      map[string]*Package
	sorter      *toposort.Sorter
	errs        *vdlutil.Errors
	vdlenv      *compile.Env
}

func newDepSorter(opts Opts, errs, warnings *vdlutil.Errors) *depSorter {
	ds := &depSorter{
		opts:      opts,
		rootDir:   RootDir(errs),
		srcDirs:   SrcDirs(errs),
		goModules: goModules(errs),
		errs:      errs,
		vdlenv:    compile.NewEnvWithErrors(errs, warnings),
	}
	ds.reset()
	// If VDLROOT isn't set, we must initialize the builtInRoot, to allow
	// "vdltool" and its dependencies to be imported.  We can't parse any config
	// files for each package yet; they are initialized later.
	var configFiles []string
	if ds.rootDir == "" {
		configFiles = ds.initBuiltInRootPackages(opts)
	}
	// Resolve the "vdltool" import and build all transitive packages into vdlenv.
	// This special env is used when building "vdl.config" files, which have the
	// implicit "vdltool" import.
	if ds.resolveImportPath("vdltool", UnknownPathIsError, "<bootstrap>") == nil {
		errs.Errorf(`can't resolve "vdltool" package`)
	}
	for _, pkg := range ds.Sort() {
		BuildPackage(pkg, ds.vdlenv)
	}
	// We can now initialize config files for the builtin root packages.
	if len(configFiles) > 0 {
		ds.initBuiltInRootConfigs(configFiles)
	}
	// We must reset back to an empty depSorter, to ensure the transitive packages
	// returned by the depSorter don't include "vdltool".
	ds.reset()
	return ds
}

func (ds *depSorter) reset() {
	ds.pathMap = make(map[string]*Package)
	ds.dirMap = make(map[string]*Package)
	ds.sorter = new(toposort.Sorter)
}

func (ds *depSorter) errorf(format string, v ...interface{}) {
	ds.errs.Errorf(format, v...)
}

// initBuiltInRootPackages initializes the built-in root packages, representing
// all standard VDLROOT packages, which are built-in to the binary.  The package
// name and config aren't initialized yet.  The name is set in addPackageAndDeps
// when the package is actually used.  The config is initialized in
// initBuiltInRootConfigs, which can only happen after ds.vdlenv is populated.
// Returns a list of the vdl.config files for later processing.
func (ds *depSorter) initBuiltInRootPackages(opts Opts) []string {
	root := make(map[string]*Package)
	var configFiles []string
	var lastDir string
	// Loop through built-in vdl files to create the root package map.
	fileNames := builtinvdlroot.AssetNames()
	sort.Strings(fileNames)
	for _, fileName := range fileNames {
		dir, base := path.Split(fileName)
		dir = path.Clean(dir)
		if base == opts.vdlConfigName() {
			configFiles = append(configFiles, fileName)
			continue
		}
		if !validFile(base) {
			ds.errorf("%s: invalid built-in vdl file", fileName)
			continue
		}
		vdlutil.Vlog.Printf("%s: adding built-in vdl file", fileName)
		if dir != lastDir {
			// Create a new built-in package and add it to the root map.
			root[dir] = &Package{
				IsRoot:        true,
				IsBuiltIn:     true,
				Path:          dir,
				GenPath:       path.Join(vdlrootImportPrefix, dir),
				Dir:           dir,
				OpenFilesFunc: openBuiltInFiles,
			}
			lastDir = dir
		}
		root[dir].BaseFileNames = append(root[dir].BaseFileNames, base)
	}
	vdlutil.Vlog.Printf("added builtin root %v", root)
	ds.builtInRoot = root
	return configFiles
}

// initBuiltInRootConfigs initializes the vdl.configs of built-in root packages.
func (ds *depSorter) initBuiltInRootConfigs(configFiles []string) {
	for _, configFile := range configFiles {
		configData, err := builtinvdlroot.Asset(configFile)
		if err != nil {
			ds.errorf("%s: can't load builtin config file: %v", configFile, err)
			continue
		}
		pkgPath, _ := path.Split(configFile)
		pkgPath = path.Clean(pkgPath)
		p := ds.builtInRoot[pkgPath]
		if p == nil {
			ds.errorf("%s: package %q doesn't exist in builtin root", configFile, pkgPath)
			continue
		}
		p.initVDLConfig(configFile, bytes.NewReader(configData), ds.vdlenv)
	}
}

// ResolvePath resolves path into package(s) and adds them to the sorter.
// Returns true iff path could be resolved.
func (ds *depSorter) ResolvePath(path string, mode UnknownPathMode) bool {
	if path == "all" {
		// Special-case "all", with the same behavior as Go.
		path = "..."
	}
	isDirPath := IsDirPath(path)
	dots := strings.Index(path, "...")
	switch {
	case dots >= 0:
		return ds.resolveWildcardPath(isDirPath, path[:dots], path[dots:])
	case isDirPath:
		return ds.resolveDirPath(path, mode) != nil
	default:
		return ds.resolveImportPath(path, mode, "<root>") != nil
	}
}

// resolveWildcardPath resolves wildcards for both dir and import paths.  The
// prefix is everything before the first "...", and the suffix is everything
// including and after the first "..."; note that multiple "..." wildcards may
// occur within the suffix.  Returns true iff any packages were resolved.
//
// The strategy is to compute one or more directories that contain everything
// that could possibly be matched, along with a filename pattern to match
// against.  Then we walk through each directory, matching against the pattern.
func (ds *depSorter) resolveWildcardPath(isDirPath bool, prefix, suffix string) bool { //nolint:gocyclo
	resolvedAny := false
	type dirAndSrc struct {
		dir, src string
	}

	var walkDirs []dirAndSrc // directories to walk through
	var pattern string       // pattern to match against, starting after root dir
	if isDirPath {
		// prefix and suffix are directory paths.
		dir, pre := filepath.Split(prefix)
		pattern = filepath.Clean(pre + suffix)
		walkDirs = append(walkDirs, dirAndSrc{filepath.Clean(dir), ""})
	} else {
		// prefix and suffix are slash-separated import paths.
		slashDir, pre := path.Split(prefix)
		pattern = filepath.Clean(pre + filepath.FromSlash(suffix))
		dir := filepath.FromSlash(slashDir)
		for _, srcDir := range ds.srcDirs {
			if gomod, ok := ds.goModules[srcDir]; ok {
				walkDirs = append(walkDirs, dirAndSrc{filepath.Join(srcDir, strings.TrimPrefix(dir, gomod)), srcDir})
				continue
			}
			walkDirs = append(walkDirs, dirAndSrc{filepath.Join(srcDir, dir), srcDir})
		}
		// Look in our built-in vdlroot for matches against standard packages.
		matcher, err := createMatcher(prefix + suffix)
		if err != nil {
			ds.errorf("%v", err)
			return false
		}
		for pkgPath, pkg := range ds.builtInRoot {
			if matcher.MatchString(pkgPath) {
				ds.addPackageAndDeps(pkg)
				resolvedAny = true
			}
		}
	}
	matcher, err := createMatcher(pattern)
	if err != nil {
		ds.errorf("%v", err)
		return false
	}

	// Walk through root dirs and subdirs, looking for matches.
	for _, walk := range walkDirs {
		goModule, isGoModule := ds.goModules[walk.dir]
		src, dir := walk.src, walk.dir
		//nolint:errcheck
		filepath.Walk(walk.dir, func(dirPath string, info os.FileInfo, err error) error {
			// Ignore errors and non-directory elements.
			if err != nil || !info.IsDir() {
				return nil
			}
			// Skip the dir and subdirs if the elem should be ignored.
			_, elem := filepath.Split(dirPath)
			if ignorePathElem(elem) {
				vdlutil.Vlog.Printf("%s: ignoring dir", dirPath)
				return filepath.SkipDir
			}
			// Special-case to skip packages with the vdlroot import prefix.  These
			// packages should only appear at the root of the package path space.
			if src != "" {
				pkgPath := strings.TrimPrefix(dirPath, src)
				pkgPath = strings.TrimPrefix(pkgPath, "/")
				if strings.HasPrefix(pkgPath, vdlrootImportPrefix) {
					return filepath.SkipDir
				}

				// Special case to handle go modules, where the current
				// directory is within a go module and hence the matching
				// for the vdlroot directory must take this into account.
				for goModRoot, goModPrefix := range ds.goModules {
					// Test for dirPath being within a go module.
					if subdir := strings.TrimPrefix(dirPath, goModRoot); subdir != dirPath {
						// Test to see if the subdir would match the vdlroot import.
						if path.Join(goModPrefix, subdir) == vdlrootImportPrefix {
							return filepath.SkipDir
						}
					}
				}

			}

			// Ignore the dir if it doesn't match our pattern.  We still process the
			// subdirs since they still might match.
			//
			// TODO(toddw): We could add an optimization to skip subdirs that can't
			// possibly match the matcher.  E.g. given pattern "a..." we can skip
			// the subdirs if the dir doesn't start with "a".
			matchPath := dirPath[len(dir):]
			matchPath = strings.TrimPrefix(matchPath, filePathSeparator)

			// Match against the raw path, and also against one with the
			// go module prefix prepended if go modules are in use.
			if !matcher.MatchString(matchPath) {
				if !isGoModule {
					return nil
				}
				// Try prepending the missing gomodule prefix to the path
				// and then match.
				if !matcher.MatchString(filepath.Join(goModule, matchPath)) {
					return nil
				}
			}

			// Finally resolve the dir.
			if ds.resolveDirPath(dirPath, UnknownPathIsIgnored) != nil {
				resolvedAny = true
			}
			return nil
		})
	}
	return resolvedAny
}

var filePathSeparator = string(filepath.Separator)

const packagePathSeparator = "/"

// createMatcher creates a regexp matcher out of the file pattern.
func createMatcher(pattern string) (*regexp.Regexp, error) {
	rePat := regexp.QuoteMeta(pattern)
	rePat = strings.ReplaceAll(rePat, `\.\.\.`, `.*`)
	// Add special-case so that x/... also matches x.
	slashDotStar := regexp.QuoteMeta(filePathSeparator) + ".*"
	if strings.HasSuffix(rePat, slashDotStar) {
		rePat = rePat[:len(rePat)-len(slashDotStar)] + "(" + slashDotStar + ")?"
	}
	rePat = `^` + rePat + `$`
	matcher, err := regexp.Compile(rePat)
	if err != nil {
		return nil, fmt.Errorf("can't compile package path regexp %s: %v", rePat, err)
	}
	return matcher, nil
}

// resolveDirPath resolves dir into a Package.  Returns the package, or nil if
// it can't be resolved.
func (ds *depSorter) resolveDirPath(dir string, mode UnknownPathMode) *Package {
	// If the package already exists in our dir map, we can just return it.
	absDir, err := filepath.Abs(dir)
	if err != nil {
		ds.errorf("%s: can't make absolute (%v)", dir, err)
	}
	if pkg := ds.dirMap[absDir]; pkg != nil {
		return pkg
	}
	// Deduce the package path, and ensure it corresponds to exactly one package.
	// We always deduce the package path from the package directory, even if we
	// originally resolved from an import path, and thus already "know" the
	// package path.  This is to ensure we correctly handle vdl standard packages.
	// E.g. if we're given "v.io/v23/vdlroot/vdltool" as an import path, the
	// resulting package path must be "vdltool".
	pkgPath, genPath, err := ds.deducePackagePath(absDir)
	if err != nil {
		ds.errorf("%s: can't deduce package path (%v)", absDir, err)
		return nil
	}
	if !validPackagePath(pkgPath) {
		mode.logOrErrorf(ds.errs, "%s: package path %q is invalid", absDir, pkgPath)
		return nil
	}

	if pkg := ds.pathMap[pkgPath]; pkg != nil {
		mode.logOrErrorf(ds.errs, "%s: package path %q already resolved from %s", absDir, pkgPath, pkg.Dir)
		return nil
	}

	// Make sure the directory really exists, and add the package and deps.
	fileInfo, err := os.Stat(absDir)
	if err != nil {
		mode.logOrErrorf(ds.errs, "%v", err)
		return nil
	}

	if !fileInfo.IsDir() {
		mode.logOrErrorf(ds.errs, "%s: package isn't a directory", absDir)
		return nil
	}
	pkg := newPackage(pkgPath, genPath, absDir, mode, ds.opts, ds.vdlenv)
	if pkg == nil {
		return nil
	}
	ds.addPackageAndDeps(pkg)
	return pkg
}

// resolveImportPath resolves pkgPath into a Package.  Returns the package, or
// nil if it can't be resolved.
func (ds *depSorter) resolveImportPath(pkgPath string, mode UnknownPathMode, prefix string) *Package {
	pkgPath = path.Clean(pkgPath)
	if pkg := ds.pathMap[pkgPath]; pkg != nil {
		return pkg
	}
	if !validPackagePath(pkgPath) {
		mode.logOrErrorf(ds.errs, "%s: import path %q is invalid", prefix, pkgPath)
		return nil
	}
	// Special-case to disallow packages under the vdlroot dir.
	if strings.HasPrefix(pkgPath, vdlrootImportPrefix) {
		mode.logOrErrorf(ds.errs, "%s: import path %q is invalid (packages under vdlroot must be specified without the vdlroot prefix)", prefix, pkgPath)
		return nil
	}
	// Look in the builtin vdlroot first, if it has been initialized.
	if pkg := ds.builtInRoot[pkgPath]; pkg != nil {
		ds.addPackageAndDeps(pkg)
		return pkg
	}
	// Look through srcDirs in-order until we find a valid package dir.
	var dirs []string
	for _, srcDir := range ds.srcDirs {
		dir := filepath.Join(srcDir, filepath.FromSlash(pkgPath))
		if pkg := ds.resolveDirPath(dir, UnknownPathIsIgnored); pkg != nil {
			vdlutil.Vlog.Printf("%s: resolved import path %q", pkg.Dir, pkgPath)
			return pkg
		}
		dirs = append(dirs, dir)

		// Try again but as a go module.
		if gomod, ok := ds.goModules[srcDir]; ok {
			dir := filepath.Join(srcDir, filepath.FromSlash(strings.TrimPrefix(pkgPath, gomod)))
			if pkg := ds.resolveDirPath(dir, UnknownPathIsIgnored); pkg != nil {
				vdlutil.Vlog.Printf("%s: resolved import path %q using go.mod to %v", pkg.Dir, pkgPath, dir)
				return pkg
			}
		}
	}
	// We can't find a valid dir corresponding to this import path.
	detail := "   " + strings.Join(dirs, "\n   ")
	mode.logOrErrorf(ds.errs, "%s: can't resolve %q in any of:\n%s", prefix, pkgPath, detail)
	return nil
}

// addPackageAndDeps adds the pkg and its dependencies to the sorter.
func (ds *depSorter) addPackageAndDeps(pkg *Package) {
	vdlutil.Vlog.Printf("%s: resolved package path %q", pkg.Dir, pkg.Path)
	ds.dirMap[pkg.Dir] = pkg
	ds.pathMap[pkg.Path] = pkg
	ds.sorter.AddNode(pkg)
	pfiles := ParsePackage(pkg, parse.Opts{ImportsOnly: true}, ds.errs)
	if pkg.Name == "" {
		pkg.Name = parse.InferPackageName(pfiles, ds.errs)
	}
	for _, pf := range pfiles {
		ds.addImportDeps(pkg, pf.Imports, filepath.Join(pkg.Path, pf.BaseName))
	}
}

// addImportDeps adds transitive dependencies represented by imports to the
// sorter.  If the pkg is non-nil, an edge is added between the pkg and its
// dependencies; otherwise each dependency is added as an independent node.
func (ds *depSorter) addImportDeps(pkg *Package, imports []*parse.Import, file string) {
	for _, imp := range imports {
		if dep := ds.resolveImportPath(imp.Path, UnknownPathIsError, file); dep != nil {
			if pkg != nil {
				ds.sorter.AddEdge(pkg, dep)
			} else {
				ds.sorter.AddNode(dep)
			}
		}
	}
}

// AddConfigDeps takes a config file represented by its file name and src data,
// and adds all transitive dependencies to the sorter.
func (ds *depSorter) AddConfigDeps(fileName string, src io.Reader) {
	if pconfig := parse.ParseConfig(fileName, src, parse.Opts{ImportsOnly: true}, ds.errs); pconfig != nil {
		ds.addImportDeps(nil, pconfig.Imports, fileName)
	}
}

// deducePackagePath deduces the package path for dir, by looking for prefix
// matches against the src dirs.  The resulting package path may be incorrect
// even if no errors are reported; see the depSorter comment for details.
func (ds *depSorter) deducePackagePath(dir string) (string, string, error) {
	for _, srcDir := range ds.srcDirs {
		if strings.HasPrefix(dir, srcDir) {
			relPath, err := filepath.Rel(srcDir, dir)
			if err != nil {
				return "", "", err
			}
			pkgPath := path.Clean(filepath.ToSlash(relPath))
			// Allow for go module directory structure.
			if gomod, ok := ds.goModules[srcDir]; ok {
				pkgPath = path.Join(gomod, pkgPath)
			}
			genPath := pkgPath
			if pre := vdlrootImportPrefix + "/"; strings.HasPrefix(pkgPath, pre) {
				// The pkgPath should never include the vdlroot prefix.
				pkgPath = pkgPath[len(pre):]
			} else if srcDir == ds.rootDir {
				// We matched against the VDLROOT dir.  The genPath needs to include the
				// vdlroot prefix.
				genPath = path.Join(vdlrootImportPrefix, pkgPath)
			}
			return pkgPath, genPath, nil
		}
	}
	return "", "", fmt.Errorf("no matching src dirs")
}

// Sort sorts all targets and returns the resulting list of Packages.
func (ds *depSorter) Sort() []*Package {
	sorted, cycles := ds.sorter.Sort()
	if len(cycles) > 0 {
		cycleStr := toposort.DumpCycles(cycles, printPackagePath)
		ds.errorf("cyclic package dependency: %v", cycleStr)
		return nil
	}
	if len(sorted) == 0 {
		return nil
	}
	targets := make([]*Package, len(sorted))
	for ix, iface := range sorted {
		targets[ix] = iface.(*Package)
	}
	return targets
}

func printPackagePath(v interface{}) string {
	return v.(*Package).Path
}

// Opts specifies additional options for collecting build information.
type Opts struct {
	// VDLConfigName specifies the name of the optional config file in each vdl
	// source package.  If empty we use "vdl.config" by default.
	VDLConfigName string
}

func (o Opts) vdlConfigName() string {
	if o.VDLConfigName != "" {
		return o.VDLConfigName
	}
	return "vdl.config"
}

// TransitivePackages takes a list of paths, and returns the corresponding
// packages and transitive dependencies, ordered by dependency.  Each path may
// either be a directory (IsDirPath) or an import (IsImportPath).
//
// A path is a pattern if it includes one or more "..." wildcards, each of which
// can match any string, including the empty string and strings containing
// slashes.  Such a pattern expands to all packages found in SrcDirs with names
// matching the pattern.  As a special-case, x/... matches x as well as x's
// subdirectories.
//
// The special-case "all" is a synonym for "...", and denotes all packages found
// in SrcDirs.
//
// Import path elements and file names are not allowed to begin with "." or "_";
// such paths are ignored in wildcard matches, and return errors if specified
// explicitly.
//
// The mode specifies whether we should ignore or produce errors for paths that
// don't resolve to any packages.  The opts arg specifies additional options.
func TransitivePackages(paths []string, mode UnknownPathMode, opts Opts, errs, warnings *vdlutil.Errors) []*Package {
	ds := newDepSorter(opts, errs, warnings)
	for _, path := range paths {
		if !ds.ResolvePath(path, mode) {
			mode.logOrErrorf(errs, "can't resolve %q to any packages", path)
		}
	}
	return ds.Sort()
}

// TransitivePackagesForConfig takes a config file represented by its file name
// and src data, and returns all package dependencies in transitive order.
//
// The opts arg specifies additional options.
func TransitivePackagesForConfig(fileName string, src io.Reader, opts Opts, errs, warnings *vdlutil.Errors) []*Package {
	ds := newDepSorter(opts, errs, warnings)
	ds.AddConfigDeps(fileName, src)
	return ds.Sort()
}

// ParsePackage parses the given pkg with the given parse opts, and returns a
// slice of parsed files, sorted by name.  Errors are reported in errs.
func ParsePackage(pkg *Package, opts parse.Opts, errs *vdlutil.Errors) (pfiles []*parse.File) {
	vdlutil.Vlog.Printf("Parsing package %s %q, dir %s", pkg.Name, pkg.Path, pkg.Dir)
	files, err := pkg.OpenFiles()
	if err != nil {
		errs.Errorf("can't open vdl files %v (%v)", pkg.BaseFileNames, err)
		return nil
	}
	for filename, src := range files {
		if pf := parse.ParseFile(path.Join(pkg.Path, filename), src, opts, errs); pf != nil {
			pfiles = append(pfiles, pf)
		}
	}
	sort.Sort(byBaseName(pfiles))
	if err := pkg.CloseFiles(); err != nil {
		vdlutil.Vlog.Printf("Parsing package %s %q, dir %s: failed to close files: %v", pkg.Name, pkg.Path, pkg.Dir, err)
	}
	return
}

// byBaseName implements sort.Interface
type byBaseName []*parse.File

func (b byBaseName) Len() int           { return len(b) }
func (b byBaseName) Less(i, j int) bool { return b[i].BaseName < b[j].BaseName }
func (b byBaseName) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }

// BuildPackage parses and compiles the given pkg, updates env with the compiled
// package and returns it.  Errors are reported in env.
//
// All imports that pkg depend on must have already been compiled and populated
// into env.
//nolint:golint // API change required.
func BuildPackage(pkg *Package, env *compile.Env) *compile.Package {
	pfiles := ParsePackage(pkg, parse.Opts{}, env.Errors)
	return compile.CompilePackage(pkg.Path, pkg.GenPath, pfiles, pkg.Config, env)
}

// BuildConfig parses and compiles the given config src and returns it.  Errors
// are reported in env; fileName is only used for error reporting.
//
// The implicit type is applied to the exported config const; untyped consts and
// composite literals with no explicit type assume the implicit type.  Errors
// are reported if the implicit type isn't assignable from the final value.  If
// the implicit type is nil, the exported config const must be explicitly typed.
//
// All packages that the config src depends on must have already been compiled
// and populated into env.  The imports are injected into the parsed src,
// behaving as if the src had listed the imports explicitly.
//nolint:golint // API change required.
func BuildConfig(fileName string, src io.Reader, implicit *vdl.Type, imports []string, env *compile.Env) *vdl.Value {
	pconfig := parse.ParseConfig(fileName, src, parse.Opts{}, env.Errors)
	if pconfig != nil {
		pconfig.AddImports(imports...)
	}
	return compile.CompileConfig(implicit, pconfig, env)
}

// BuildConfigValue is a convenience function that runs BuildConfig, and then
// converts the result into value.  The implicit type used by BuildConfig is
// inferred from the value.
//nolint:golint // API change required.
func BuildConfigValue(fileName string, src io.Reader, imports []string, env *compile.Env, value interface{}) {
	rv := reflect.ValueOf(value)
	tt, err := vdl.TypeFromReflect(rv.Type())
	if err != nil {
		env.Errors.Errorf(err.Error())
		return
	}
	if tt.Kind() == vdl.Optional {
		// The value is typically a Go pointer, which translates into VDL optional.
		// Remove the optional when determining the implicit type for BuildConfig.
		tt = tt.Elem()
	}
	vconfig := BuildConfig(fileName, src, tt, imports, env)
	if vconfig == nil {
		return
	}
	if err := vdl.ConvertReflect(rv, reflect.ValueOf(vconfig)); err != nil {
		env.Errors.Errorf("can't convert to %T from %v (%v)", value, vconfig, err)
		return
	}
}

// BuildExprs parses and compiles the given data into a slice of values.  The
// input data is specified in VDL syntax, with commas separating multiple
// expressions.  There must be at least one expression specified in data.
// Errors are reported in env.
//
// The given types specify the type of each returned value with the same slice
// position.  If there are more types than returned values, the extra types are
// ignored.  If there are fewer types than returned values, the last type is
// used for all remaining values.  Nil entries in types are allowed, and
// indicate that the expression itself must be fully typed.
//
// All imports that the input data depends on must have already been compiled
// and populated into env.
//nolint:golint // API change required.
func BuildExprs(data string, types []*vdl.Type, env *compile.Env) []*vdl.Value {
	var values []*vdl.Value
	var t *vdl.Type
	for ix, pexpr := range parse.ParseExprs(data, env.Errors) {
		if ix < len(types) {
			t = types[ix]
		}
		values = append(values, compile.CompileExpr(t, pexpr, env))
	}
	return values
}

// Boostrapping mode is required to generate all vdl output files starting
// with no pre-existing vdl generated files. The vdl command only needs to
// generate go output when boostrapping in order to regenerate
// v23/vdlroot/vdltool and v23/vdl.
//
//go:build vdlbootstrapping
// +build vdlbootstrapping

package main

import (
	"fmt"

	"v.io/v23/vdlroot/vdltool"
	"v.io/x/ref/lib/vdl/build"
	"v.io/x/ref/lib/vdl/compile"
)

var (
	genLangAll = genLangs([]vdltool.GenLanguage{vdltool.GenLanguageGo})
)

func init() {
	// Options for audit are identical to generate.
	cmdAudit.Flags = cmdGenerate.Flags
}

func genLanguageFromString(lang string) (vdltool.GenLanguage, error) {
	if lang != "go" {
		return vdltool.GenLanguage(0), fmt.Errorf("only go is supported in bootstrapping mode")
	}
	return vdltool.GenLanguageGo, nil
}

func shouldGenerate(config vdltool.Config, lang vdltool.GenLanguage) bool {
	// Only go should be generated when bootstrapping.
	return lang == vdltool.GenLanguageGo
}

func handleLanguages(gl vdltool.GenLanguage, target *build.Package, audit bool, pkg *compile.Package, env *compile.Env, pathToDir map[string]string) bool {
	if gl == vdltool.GenLanguageGo {
		return handleGo(audit, pkg, env, target)
	}
	env.Errors.Errorf("Generating code for language %v isn't supported", gl)
	return false
}

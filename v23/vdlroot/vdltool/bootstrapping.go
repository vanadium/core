// Package vdltool contains the VDL data structures used to configure
// the behaviour of the vdltool itself. This introduces a circular dependency
// which needs to be managed carefully when 'bootstrapping', i.e. building
// the vdl tool chain without these packages being already genrated.
//
// Boostrapping is required to generate all vdl output files starting with no
// pre-existing vdl generated files. Currently the vdl tool
// chain depends on the:
//   1. generated output of this package (v.io/v23/vdlroot/vdltool) and
//      specifically the vdltool.Config data type used to configure
//      vdl native types.
//   2. the VDL types in v.io/v23/vdl, which require support for native types.
//   3. the 'builtin' VDL files and packages in v.io/v23/vdlroot/{math,signature,time},
//      which also requre support for native types.
//
// Two build tags are required:
//   1. vdltoolbootstrapping which allows for the vdltool tool to be run
//      with empty implementations of the data types in vdltool in order
//      to generate the correct output in the vdltool directory. This tag
//      essentially breaks the dependency on the vdltool directory during
//      bootstrapping. Once the command below is run, the vdltool directory
//      is completely generated.
//      	VDLROOT=$(pwd) go run -tags vdltoolbootstrapping \
//          v.io/x/ref/cmd/vdl generate --lang=go  ./vdltool
//   2. vdlbootstrapping allows for bootstrapping the remaining packages
//      in v.io/v23/vdlroot and v.io/v23/vdl and the tag controls defining
//      internal versions of the data types produced by these packages and
//      only enabling go code generation.
//      Once the command below is run all of these packages are now completely
//      generated.
//      	VDLROOT=$(pwd) go run -tags vdlbootstrapping \
//			v.io/x/ref/cmd/vdl generate --lang=go \
//			./math ./time ./signature v.io/v23/vdl
//

//go:build vdltoolbootstrapping
// +build vdltoolbootstrapping

package vdltool

// GenLanguage represents the languages that can be generated when bootstrapping.
type GenLanguage int

const (
	// GenLanguageGo is the only language that can be generated when bootstrapping.
	GenLanguageGo GenLanguage = 0
)

// String implements stringer.
func (GenLanguage) String() string {
	return "go"
}

// Config is the minimal, ie. empty, config that can be used whilst
// bootstrapping.
type Config struct{}

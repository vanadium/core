// Boostrapping mode is required to generate all vdl output files starting
// with no pre-existing vdl generated files. Currently the vdl tool
// chain depends on the generated output of this package and v.io/v23/vdl.
//
// The vdltoolbootstrapping ands vdlbootstrapping tags are required when
// generating the vdltool .vdl.go output. The vdlbootstrapping tag is required
// by the v.io/v23/vdl package to use a dummy implementation of vdl.WireError
// and associated types.
//
// export VDLROOT=$$(pwd)/v23/vdlroot ; \
//	cd v23/vdlroot && \
//	go run -tags vdltoolbootstrapping,vdlbootstrapping \
//		 v.io/x/ref/cmd/vdl generate --lang=go ./vdltool
//
// Note that two build tags are required to allow for the separation of the
// generated output of this package and bootstrapping in other packages and
// in particular the vdl command-line tool and code generation.
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

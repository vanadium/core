package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"v.io/x/lib/gosh"
)

type tb struct{}

func (t *tb) FailNow() {
	log.Panicf("build failed")
}
func (t *tb) Logf(format string, args ...interface{}) {
	log.Printf(format, args...)
}

var (
	packageName   string
	outputDirName string
	outputLibName string
)

func init() {
	flag.StringVar(&packageName, "package-name", "v.io/x/jni/main", "build the specified package as a .so/.dylib for use with java")
	flag.StringVar(&outputDirName, "output-dir", "", "the directory for the output file")
	flag.StringVar(&outputLibName, "output-lib", "libv23", "the prefix for the output file, .so or .dylib will be appended appropriately")
}

func main() {
	flag.Parse()

	javaHome := os.Getenv("JAVA_HOME")
	if len(javaHome) == 0 {
		log.Fatalf("JAVA_HOME not specified")
	}
	if len(outputDirName) == 0 {
		log.Fatalf("--output-dir not specified")
	}

	if runtime.GOOS == "darwin" {
		outputLibName += ".dylib"
	} else {
		outputLibName += ".so"
	}

	sh := gosh.NewShell(&tb{})
	sh.PropagateChildOutput = true
	defer sh.Cleanup()

	cmd := sh.Cmd(
		"go",
		"build",
		"-buildmode=c-shared",
		"-v",
		"-tags", "java cgo",
		"-o", filepath.Join(outputDirName, outputLibName),
		packageName)
	cmd.Vars["CGO_CFLAGS"] = fmt.Sprintf("-I%v/include -I%v/include/%v", javaHome, javaHome, runtime.GOOS)
	cmd.Run()
}

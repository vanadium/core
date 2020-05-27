// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package dirprinter contains utilities for dumping the contents and structure
// of a directory tree.
package dirprinter

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	// \u2502 is Box Drawings Light Vertical.
	vertLine = '\u2502'
	// \u2514 is Box Drawings Light Up And Right.
	upRightLine = '\u2514'
	// \u251c is Box Drawings Light Vertical And Right.
	vertRightLine = '\u251c'
	// \u2500 is Box Drawings Light Horizontal.
	horizLine = '\u2500'
)

// computeLastChild traverses dir and returns a map containing the descendants
// of dir that are the last entry in their respective parents.
func computeLastChild(dir string) (map[string]bool, error) {
	lastChild := make(map[string]string)
	setChild := func(path string, _ os.FileInfo, _ error) error {
		lastChild[filepath.Dir(path)] = path
		return nil
	}
	if err := filepath.Walk(dir, setChild); err != nil {
		return nil, err
	}
	isLastChild := make(map[string]bool)
	for _, p := range lastChild {
		isLastChild[p] = true
	}
	return isLastChild, nil
}

// printDirTree traverses dir and pretty-prints the tree under dir.
func printDirTree(w io.Writer, dir string) error {
	isLastChild, err := computeLastChild(dir)
	if err != nil {
		return err
	}
	printFn := func(path string, info os.FileInfo, err error) error {
		me := path
		var graphics []rune
		prepend := func(p []rune, r ...rune) []rune {
			return append(r, p...)
		}
		if err != nil {
			// If the entry has an error, we want to print the error
			// at the appropriate place in the tree (right below the
			// entry, with the appropriate graphics preceding it).
			me = filepath.Join(me, "dummy")
		}
		for me != dir {
			parent, _ := filepath.Split(me)
			me = filepath.Clean(parent)
			if me == dir {
				break
			}
			graphics = prepend(graphics, ' ', ' ')
			if isLastChild[me] {
				graphics = prepend(graphics, ' ')
			} else {
				graphics = prepend(graphics, vertLine)
			}
		}
		if err != nil {
			// Replace trailing |<space><space> with
			// |-- in the case of an error.
			fmt.Fprint(w, string(graphics[:len(graphics)-3]))
			fmt.Fprint(w, string(append([]rune{vertRightLine}, horizLine, horizLine)))
			printEntry(w, info)
			fmt.Fprint(w, "\n")
			fmt.Fprint(w, string(graphics))
			fmt.Fprintf(w, "ERROR: %v\n", err)
			return filepath.SkipDir
		}
		if filepath.Clean(path) != dir {
			if isLastChild[path] {
				graphics = append(graphics, upRightLine)
			} else {
				graphics = append(graphics, vertRightLine)
			}
			graphics = append(graphics, horizLine, horizLine)
		}
		fmt.Fprint(w, string(graphics))
		printEntry(w, info)
		fmt.Fprint(w, "\n")
		return nil
	}

	return filepath.Walk(dir, printFn)
}

// printEntry is used by printDirTree to show a summary of a directory entry.
func printEntry(w io.Writer, info os.FileInfo) {
	fmt.Fprint(w, info.Name())
	if info.IsDir() {
		fmt.Fprint(w, string(filepath.Separator))
	} else {
		fmt.Fprintf(w, " %dB", info.Size())
	}
	fmt.Fprintf(w, " %v", info.ModTime().Format("15:04:05.000000"))
}

// printDirContents traverses the given dir and prints out information about
// descendant files.  If the file is text, it also dumps the contents out.
func printDirContents(w io.Writer, dir string) error {
	printFn := func(path string, info os.FileInfo, _ error) error {
		if info != nil && info.IsDir() {
			return nil
		}
		path = filepath.Clean(path)
		printPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		bannerSep := strings.Repeat(string(horizLine), len(printPath))
		fmt.Fprintln(w, bannerSep)
		fmt.Fprintln(w, printPath)
		fmt.Fprintln(w, bannerSep)
		if err := dumpFile(w, path, info); err != nil {
			fmt.Fprintf(w, "ERROR: %v\n", err)
		}
		return nil
	}
	return filepath.Walk(dir, printFn)
}

// dumpFile prints out information about the file at path.
func dumpFile(w io.Writer, path string, info os.FileInfo) error {
	switch {
	case info.Mode().IsRegular():
		return dumpRegular(w, path)
	case info.Mode()&os.ModeSymlink != 0:
		target, err := filepath.EvalSymlinks(path)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "Link to: %s\n", target)
		return nil
	default:
		fmt.Fprintf(w, "MODE: %x\n", info.Mode())
		return nil
	}
}

// dumpRegular prints out information about the regular file at path.  If the
// content type is text, it dumps out the contents of the file.
func dumpRegular(w io.Writer, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	data := make([]byte, 512)
	count, err := io.ReadAtLeast(f, data, 512)
	if err != nil && err != io.ErrUnexpectedEOF {
		return err
	}
	data = data[:count]
	contentType := http.DetectContentType(data)
	fmt.Fprintln(w, "Content-type:", contentType)
	if strings.HasPrefix(contentType, "text") {
		if _, err := w.Write(data); err != nil {
			return err
		}
		io.Copy(w, f) //nolint:errcheck
		fmt.Fprintln(w)
	}
	return nil
}

// DumpDir dumps the dir's structure and files.
func DumpDir(w io.Writer, dir string) error {
	dir = filepath.Clean(dir)
	if err := printDirTree(w, dir); err != nil {
		return err
	}
	return printDirContents(w, dir)
}

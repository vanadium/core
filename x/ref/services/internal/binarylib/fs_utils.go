// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package binarylib

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"v.io/v23/context"
	"v.io/v23/verror"
)

const (
	checksumFileName  = "checksum"
	dataFileName      = "data"
	lockFileName      = "lock"
	nameFileName      = "name"
	mediaInfoFileName = "mediainfo"
)

// checksumExists checks whether the given part path is valid and
// contains a checksum. The implementation uses the existence of
// the path dir to determine whether the part is valid, and the
// existence of checksum to determine whether the binary part
// exists.
func checksumExists(ctx *context.T, path string) error {
	switch _, err := os.Stat(path); {
	case os.IsNotExist(err):
		return verror.New(ErrInvalidPart, nil, path)
	case err != nil:
		ctx.Errorf("Stat(%v) failed: %v", path, err)
		return verror.New(ErrOperationFailed, nil, path)
	}
	checksumFile := filepath.Join(path, checksumFileName)
	_, err := os.Stat(checksumFile)
	switch {
	case os.IsNotExist(err):
		return verror.New(verror.ErrNoExist, nil, path)
	case err != nil:
		ctx.Errorf("Stat(%v) failed: %v", checksumFile, err)
		return verror.New(ErrOperationFailed, nil, path)
	default:
		return nil
	}
}

// generatePartPath generates a path for the given binary part.
func (i *binaryService) generatePartPath(part int) string {
	return generatePartPath(i.path, part)
}

func generatePartPath(dir string, part int) string {
	return filepath.Join(dir, fmt.Sprintf("%d", part))
}

// getParts returns a collection of paths to the parts of the binary.
func getParts(ctx *context.T, path string) ([]string, error) {
	infos, err := ioutil.ReadDir(path)
	if err != nil {
		ctx.Errorf("ReadDir(%v) failed: %v", path, err)
		return []string{}, verror.New(ErrOperationFailed, nil, path)
	}
	nDirs := 0
	for _, info := range infos {
		if info.IsDir() {
			nDirs++
		}
	}
	result := make([]string, nDirs)
	for _, info := range infos {
		if info.IsDir() {
			partName := info.Name()
			idx, err := strconv.Atoi(partName)
			if err != nil {
				ctx.Errorf("Atoi(%v) failed: %v", partName, err)
				return []string{}, verror.New(ErrOperationFailed, nil, path)
			}
			if idx < 0 || idx >= len(infos) || result[idx] != "" {
				return []string{}, verror.New(ErrOperationFailed, nil, path)
			}
			result[idx] = filepath.Join(path, partName)
		} else {
			if info.Name() == nameFileName || info.Name() == mediaInfoFileName {
				continue
			}
			// The only entries should correspond to the part dirs.
			return []string{}, verror.New(ErrOperationFailed, nil, path)
		}
	}
	return result, nil
}

// createObjectNameTree returns a tree of all the valid object names in the
// repository.
func (i *binaryService) createObjectNameTree() *treeNode {
	pattern := i.state.rootDir
	for d := 0; d < i.state.depth; d++ {
		pattern = filepath.Join(pattern, "*")
	}
	pattern = filepath.Join(pattern, "*", nameFileName)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil
	}
	tree := newTreeNode()
	for _, m := range matches {
		name, err := ioutil.ReadFile(m)
		if err != nil {
			continue
		}
		elems := strings.Split(string(name), string(filepath.Separator))
		tree.find(elems, true)
	}
	return tree
}

type treeNode struct {
	children map[string]*treeNode
}

func newTreeNode() *treeNode {
	return &treeNode{children: make(map[string]*treeNode)}
}

func (n *treeNode) find(names []string, create bool) *treeNode {
	for {
		if len(names) == 0 {
			return n
		}
		if next, ok := n.children[names[0]]; ok {
			n = next
			names = names[1:]
			continue
		}
		if create {
			nn := newTreeNode()
			n.children[names[0]] = nn
			n = nn
			names = names[1:]
			continue
		}
		return nil
	}
}

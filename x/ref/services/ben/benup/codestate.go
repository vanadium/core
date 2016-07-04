// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"sort"

	"v.io/jiri"
	"v.io/jiri/gitutil"
	"v.io/jiri/project"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/services/ben"
)

type jiriState struct {
	Project  project.Project
	Pristine bool
	Error    error
}

func detectSourceCode(inenv *cmdline.Env) (ben.SourceCode, error) {
	// jiri.NewX seems to consume from env.Stdin, so avoid that.
	env := new(cmdline.Env)
	*env = *inenv
	env.Stdin = nil

	// For now, assume that all benchmarks are run from code inside
	// JIRI_ROOT. At some point, might want to get fancier and do things like:
	// "If os.Getwd() is not inside JIRI_ROOT but is inside some git repository,
	// then use the commit hash of that git repository instead".
	jirix, err := jiri.NewX(env)
	if err != nil {
		return "", err
	}
	projects, err := project.LocalProjects(jirix, project.FastScan)
	if err != nil {
		return "", err
	}
	ch := make(chan jiriState)
	for _, p := range projects {
		go sendJiriState(ch, jirix, p)
	}
	var dirty []string
	var manifest project.Manifest
	for i := 0; i < len(projects); i++ {
		rs := <-ch
		if err == nil {
			err = rs.Error
		}
		manifest.Projects = append(manifest.Projects, rs.Project)
		if !rs.Pristine {
			dirty = append(dirty, rs.Project.Name)
		}
	}
	if err != nil {
		return "", err
	}
	sort.Strings(dirty)
	sort.Stable(projectList(manifest.Projects))
	byts, err := manifest.ToBytes()
	if err != nil {
		return "", err
	}
	desc := fmt.Sprintf("%s", byts)
	if len(dirty) > 0 {
		desc += "<!-- PROJECTS DEVIATING FROM REVISION -->\n"
		for _, d := range dirty {
			desc += fmt.Sprintf("<!-- %s -->\n", d)
		}
	}
	return ben.SourceCode(desc), nil
}

type projectList []project.Project

func (l projectList) Len() int           { return len(l) }
func (l projectList) Less(i, j int) bool { return l[i].Name < l[j].Name }
func (l projectList) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }

func sendJiriState(out chan<- jiriState, jirix *jiri.X, p project.Project) {
	scm := gitutil.New(jirix.NewSeq(), gitutil.RootDirOpt(p.Path))
	ret := jiriState{Project: p}
	ret.Project.Revision, ret.Error = scm.CurrentRevision()
	// If there are any errors determining whether or not the source
	// deviates from the revision, treat it as being not pristine, instead
	// of an error.
	uncommitted, err := scm.HasUncommittedChanges()
	ret.Pristine = !uncommitted && err == nil
	out <- ret
}

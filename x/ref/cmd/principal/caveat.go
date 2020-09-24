// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"strings"

	"v.io/v23/security"
	"v.io/v23/vdl"
	"v.io/x/ref/lib/vdl/build"
	"v.io/x/ref/lib/vdl/compile"
	"v.io/x/ref/lib/vdl/parse"
)

// caveatsFlag defines a flag.Value for receiving multiple caveat definitions.
type caveatsFlag struct {
	caveatInfos []caveatInfo
}

type caveatInfo struct {
	expr, params string
}

// Implements flag.Value.Get
func (c caveatsFlag) Get() interface{} {
	return c.caveatInfos
}

// Implements flag.Value.Set
// Set expects s to be of the form:
// caveatExpr=VDLExpressionOfParam
func (c *caveatsFlag) Set(s string) error {
	exprAndParam := strings.SplitN(s, "=", 2)
	if len(exprAndParam) != 2 {
		return fmt.Errorf("incorrect caveat format: %s", s)
	}

	c.caveatInfos = append(c.caveatInfos, caveatInfo{exprAndParam[0], exprAndParam[1]})
	return nil
}

// Implements flag.Value.String
func (c caveatsFlag) String() string {
	return fmt.Sprint(c.caveatInfos)
}

func (c caveatsFlag) Compile() ([]security.Caveat, error) {
	if len(c.caveatInfos) == 0 {
		return nil, nil
	}
	env := compile.NewEnv(-1)
	if err := buildPackages(c.caveatInfos, env); err != nil {
		return nil, err
	}
	var caveats []security.Caveat
	for _, info := range c.caveatInfos {
		caveat, err := newCaveat(info, env)
		if err != nil {
			return nil, err
		}
		caveats = append(caveats, caveat)
	}
	return caveats, nil
}

func buildPackages(caveatInfos []caveatInfo, env *compile.Env) error {
	var (
		pkgNames []string
		exprs    []string
	)
	for _, info := range caveatInfos {
		exprs = append(exprs, info.expr, info.params)
	}
	for _, pexpr := range parse.ParseExprs(strings.Join(exprs, ","), env.Errors) {
		pkgNames = append(pkgNames, parse.ExtractExprPackagePaths(pexpr)...)
	}
	if !env.Errors.IsEmpty() {
		return fmt.Errorf("can't build expressions %v:\n%v", exprs, env.Errors)
	}
	pkgs := build.TransitivePackages(pkgNames, build.UnknownPathIsError, build.Opts{}, env.Errors, env.Warnings)
	if !env.Errors.IsEmpty() {
		return fmt.Errorf("failed to get transitive packages %v: %s", pkgNames, env.Errors)
	}
	for _, p := range pkgs {
		if build.BuildPackage(p, env); !env.Errors.IsEmpty() {
			return fmt.Errorf("failed to build package(%v): %v", p, env.Errors)
		}
	}
	return nil
}

func newCaveat(info caveatInfo, env *compile.Env) (security.Caveat, error) {
	caveatDesc, err := compileCaveatDesc(info.expr, env)
	if err != nil {
		return security.Caveat{}, err
	}
	param, err := compileParams(info.params, caveatDesc.ParamType, env)
	if err != nil {
		return security.Caveat{}, err
	}
	return security.NewCaveat(caveatDesc, param)
}

func compileCaveatDesc(expr string, env *compile.Env) (security.CaveatDescriptor, error) {
	vdlValues := build.BuildExprs(expr, []*vdl.Type{vdl.TypeOf(security.CaveatDescriptor{})}, env)
	if err := env.Errors.ToError(); err != nil {
		return security.CaveatDescriptor{}, fmt.Errorf("can't build caveat desc %s:\n%v", expr, err)
	}
	if len(vdlValues) == 0 {
		return security.CaveatDescriptor{}, fmt.Errorf("no caveat descriptors were built")
	}
	var desc security.CaveatDescriptor
	if err := vdl.Convert(&desc, vdlValues[0]); err != nil {
		return security.CaveatDescriptor{}, err
	}
	return desc, nil
}

func compileParams(paramData string, vdlType *vdl.Type, env *compile.Env) (interface{}, error) {
	params := build.BuildExprs(paramData, []*vdl.Type{vdlType}, env)
	if !env.Errors.IsEmpty() {
		return nil, fmt.Errorf("can't build param data %s:\n%v", paramData, env.Errors)
	}
	if len(params) == 0 {
		return security.CaveatDescriptor{}, fmt.Errorf("no caveat params were built")
	}
	return params[0], nil
}

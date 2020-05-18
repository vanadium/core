// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Package: groups

// Package groups defines interfaces for managing access control groups.  Groups
// can be referenced by BlessingPatterns (e.g. in AccessLists).
package groups

import (
	"fmt"
	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/i18n"
	"v.io/v23/rpc"
	"v.io/v23/security/access"
	"v.io/v23/services/permissions"
	"v.io/v23/vdl"
	"v.io/v23/verror"
)

var _ = __VDLInit() // Must be first; see __VDLInit comments for details.

//////////////////////////////////////////////////
// Type definitions

// BlessingPatternChunk is a substring of a BlessingPattern. As with
// BlessingPatterns, BlessingPatternChunks may contain references to groups.
// However, they may be restricted in other ways. For example, in the future
// BlessingPatterns may support "$" terminators, but these may be disallowed for
// BlessingPatternChunks.
type BlessingPatternChunk string

func (BlessingPatternChunk) VDLReflect(struct {
	Name string `vdl:"v.io/v23/services/groups.BlessingPatternChunk"`
}) {
}

func (x BlessingPatternChunk) VDLIsZero() bool {
	return x == ""
}

func (x BlessingPatternChunk) VDLWrite(enc vdl.Encoder) error {
	if err := enc.WriteValueString(__VDLType_string_1, string(x)); err != nil {
		return err
	}
	return nil
}

func (x *BlessingPatternChunk) VDLRead(dec vdl.Decoder) error {
	switch value, err := dec.ReadValueString(); {
	case err != nil:
		return err
	default:
		*x = BlessingPatternChunk(value)
	}
	return nil
}

type GetRequest struct {
}

func (GetRequest) VDLReflect(struct {
	Name string `vdl:"v.io/v23/services/groups.GetRequest"`
}) {
}

func (x GetRequest) VDLIsZero() bool {
	return x == GetRequest{}
}

func (x GetRequest) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(__VDLType_struct_2); err != nil {
		return err
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *GetRequest) VDLRead(dec vdl.Decoder) error {
	*x = GetRequest{}
	if err := dec.StartValue(__VDLType_struct_2); err != nil {
		return err
	}
	decType := dec.Type()
	for {
		index, err := dec.NextField()
		switch {
		case err != nil:
			return err
		case index == -1:
			return dec.FinishValue()
		}
		if decType != __VDLType_struct_2 {
			index = __VDLType_struct_2.FieldIndexByName(decType.Field(index).Name)
			if index == -1 {
				if err := dec.SkipValue(); err != nil {
					return err
				}
				continue
			}
		}
		switch index {
		}
	}
}

type GetResponse struct {
	Entries map[BlessingPatternChunk]struct{}
}

func (GetResponse) VDLReflect(struct {
	Name string `vdl:"v.io/v23/services/groups.GetResponse"`
}) {
}

func (x GetResponse) VDLIsZero() bool {
	return len(x.Entries) == 0
}

func (x GetResponse) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(__VDLType_struct_3); err != nil {
		return err
	}
	if len(x.Entries) != 0 {
		if err := enc.NextField(0); err != nil {
			return err
		}
		if err := __VDLWriteAnon_set_1(enc, x.Entries); err != nil {
			return err
		}
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func __VDLWriteAnon_set_1(enc vdl.Encoder, x map[BlessingPatternChunk]struct{}) error {
	if err := enc.StartValue(__VDLType_set_4); err != nil {
		return err
	}
	if err := enc.SetLenHint(len(x)); err != nil {
		return err
	}
	for key := range x {
		if err := enc.NextEntryValueString(__VDLType_string_1, string(key)); err != nil {
			return err
		}
	}
	if err := enc.NextEntry(true); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *GetResponse) VDLRead(dec vdl.Decoder) error {
	*x = GetResponse{}
	if err := dec.StartValue(__VDLType_struct_3); err != nil {
		return err
	}
	decType := dec.Type()
	for {
		index, err := dec.NextField()
		switch {
		case err != nil:
			return err
		case index == -1:
			return dec.FinishValue()
		}
		if decType != __VDLType_struct_3 {
			index = __VDLType_struct_3.FieldIndexByName(decType.Field(index).Name)
			if index == -1 {
				if err := dec.SkipValue(); err != nil {
					return err
				}
				continue
			}
		}
		switch index {
		case 0:
			if err := __VDLReadAnon_set_1(dec, &x.Entries); err != nil {
				return err
			}
		}
	}
}

func __VDLReadAnon_set_1(dec vdl.Decoder, x *map[BlessingPatternChunk]struct{}) error {
	if err := dec.StartValue(__VDLType_set_4); err != nil {
		return err
	}
	var tmpMap map[BlessingPatternChunk]struct{}
	if len := dec.LenHint(); len > 0 {
		tmpMap = make(map[BlessingPatternChunk]struct{}, len)
	}
	for {
		switch done, key, err := dec.NextEntryValueString(); {
		case err != nil:
			return err
		case done:
			*x = tmpMap
			return dec.FinishValue()
		default:
			if tmpMap == nil {
				tmpMap = make(map[BlessingPatternChunk]struct{})
			}
			tmpMap[BlessingPatternChunk(key)] = struct{}{}
		}
	}
}

// ApproximationType defines the type of approximation desired when a Relate
// call encounters an error (inaccessible or undefined group in a blessing
// pattern, cyclic group definitions, storage errors, invalid patterns
// etc). "Under" is used for blessing patterns in "Allow" clauses in an
// AccessList, while "Over" is used for blessing patterns in "Deny" clauses.
type ApproximationType int

const (
	ApproximationTypeUnder ApproximationType = iota
	ApproximationTypeOver
)

// ApproximationTypeAll holds all labels for ApproximationType.
var ApproximationTypeAll = [...]ApproximationType{ApproximationTypeUnder, ApproximationTypeOver}

// ApproximationTypeFromString creates a ApproximationType from a string label.
// nolint: deadcode, unused
func ApproximationTypeFromString(label string) (x ApproximationType, err error) {
	err = x.Set(label)
	return
}

// Set assigns label to x.
func (x *ApproximationType) Set(label string) error {
	switch label {
	case "Under", "under":
		*x = ApproximationTypeUnder
		return nil
	case "Over", "over":
		*x = ApproximationTypeOver
		return nil
	}
	*x = -1
	return fmt.Errorf("unknown label %q in groups.ApproximationType", label)
}

// String returns the string label of x.
func (x ApproximationType) String() string {
	switch x {
	case ApproximationTypeUnder:
		return "Under"
	case ApproximationTypeOver:
		return "Over"
	}
	return ""
}

func (ApproximationType) VDLReflect(struct {
	Name string `vdl:"v.io/v23/services/groups.ApproximationType"`
	Enum struct{ Under, Over string }
}) {
}

func (x ApproximationType) VDLIsZero() bool {
	return x == ApproximationTypeUnder
}

func (x ApproximationType) VDLWrite(enc vdl.Encoder) error {
	if err := enc.WriteValueString(__VDLType_enum_5, x.String()); err != nil {
		return err
	}
	return nil
}

func (x *ApproximationType) VDLRead(dec vdl.Decoder) error {
	switch value, err := dec.ReadValueString(); {
	case err != nil:
		return err
	default:
		if err := x.Set(value); err != nil {
			return err
		}
	}
	return nil
}

// Approximation contains information about membership approximations made
// during a Relate call.
type Approximation struct {
	Reason  string
	Details string
}

func (Approximation) VDLReflect(struct {
	Name string `vdl:"v.io/v23/services/groups.Approximation"`
}) {
}

func (x Approximation) VDLIsZero() bool {
	return x == Approximation{}
}

func (x Approximation) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(__VDLType_struct_6); err != nil {
		return err
	}
	if x.Reason != "" {
		if err := enc.NextFieldValueString(0, vdl.StringType, x.Reason); err != nil {
			return err
		}
	}
	if x.Details != "" {
		if err := enc.NextFieldValueString(1, vdl.StringType, x.Details); err != nil {
			return err
		}
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *Approximation) VDLRead(dec vdl.Decoder) error {
	*x = Approximation{}
	if err := dec.StartValue(__VDLType_struct_6); err != nil {
		return err
	}
	decType := dec.Type()
	for {
		index, err := dec.NextField()
		switch {
		case err != nil:
			return err
		case index == -1:
			return dec.FinishValue()
		}
		if decType != __VDLType_struct_6 {
			index = __VDLType_struct_6.FieldIndexByName(decType.Field(index).Name)
			if index == -1 {
				if err := dec.SkipValue(); err != nil {
					return err
				}
				continue
			}
		}
		switch index {
		case 0:
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				x.Reason = value
			}
		case 1:
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				x.Details = value
			}
		}
	}
}

//////////////////////////////////////////////////
// Error definitions

var (
	ErrNoBlessings         = verror.Register("v.io/v23/services/groups.NoBlessings", verror.NoRetry, "{1:}{2:} No blessings recognized; cannot create group Permissions")
	ErrExcessiveContention = verror.Register("v.io/v23/services/groups.ExcessiveContention", verror.RetryBackoff, "{1:}{2:} Gave up after encountering excessive contention; try again later")
	ErrCycleFound          = verror.Register("v.io/v23/services/groups.CycleFound", verror.NoRetry, "{1:}{2:} Found cycle in group definitions{:_}")
)

// NewErrNoBlessings returns an error with the ErrNoBlessings ID.
func NewErrNoBlessings(ctx *context.T) error {
	return verror.New(ErrNoBlessings, ctx)
}

// NewErrExcessiveContention returns an error with the ErrExcessiveContention ID.
func NewErrExcessiveContention(ctx *context.T) error {
	return verror.New(ErrExcessiveContention, ctx)
}

// NewErrCycleFound returns an error with the ErrCycleFound ID.
func NewErrCycleFound(ctx *context.T) error {
	return verror.New(ErrCycleFound, ctx)
}

//////////////////////////////////////////////////
// Interface definitions

// GroupReaderClientMethods is the client interface
// containing GroupReader methods.
//
// GroupReader implements methods to read or query a group's membership
// information.
type GroupReaderClientMethods interface {
	// Relate determines the relationships between the provided blessing
	// names and the members of the group.
	//
	// Given an input set of blessing names and a group defined by a set of
	// blessing patterns S, for each blessing name B in the input, Relate(B)
	// returns a set of "remainders" consisting of every blessing name B"
	// such that there exists some B' for which B = B' B" and B' is in S,
	// and "" if B is a member of S.
	//
	// For example, if a group is defined as S = {n1, n1:n2, n1:n2:n3}, then
	// Relate(n1:n2) = {n2, ""}.
	//
	// reqVersion specifies the expected version of the group's membership
	// information. If this version is set and matches the Group's current
	// version, the response will indicate that fact but will otherwise be
	// empty.
	//
	// visitedGroups is the set of groups already visited in a particular
	// chain of Relate calls, and is used to detect the presence of
	// cycles. When a cycle is detected, it is treated just like any other
	// error, and the result is approximated.
	//
	// Relate also returns information about all the errors encountered that
	// resulted in approximations, if any.
	//
	// TODO(hpucha): scrub "Approximation" for preserving privacy. Flesh
	// versioning out further. Other args we may need: option to Get() the
	// membership set when allowed (to avoid an extra RPC), options related
	// to caching this information.
	Relate(_ *context.T, blessings map[string]struct{}, hint ApproximationType, reqVersion string, visitedGroups map[string]struct{}, _ ...rpc.CallOpt) (remainder map[string]struct{}, approximations []Approximation, version string, _ error)
	// Get returns all entries in the group.
	// TODO(sadovsky): Flesh out this API.
	Get(_ *context.T, req GetRequest, reqVersion string, _ ...rpc.CallOpt) (res GetResponse, version string, _ error)
}

// GroupReaderClientStub adds universal methods to GroupReaderClientMethods.
type GroupReaderClientStub interface {
	GroupReaderClientMethods
	rpc.UniversalServiceMethods
}

// GroupReaderClient returns a client stub for GroupReader.
func GroupReaderClient(name string) GroupReaderClientStub {
	return implGroupReaderClientStub{name}
}

type implGroupReaderClientStub struct {
	name string
}

func (c implGroupReaderClientStub) Relate(ctx *context.T, i0 map[string]struct{}, i1 ApproximationType, i2 string, i3 map[string]struct{}, opts ...rpc.CallOpt) (o0 map[string]struct{}, o1 []Approximation, o2 string, err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Relate", []interface{}{i0, i1, i2, i3}, []interface{}{&o0, &o1, &o2}, opts...)
	return
}

func (c implGroupReaderClientStub) Get(ctx *context.T, i0 GetRequest, i1 string, opts ...rpc.CallOpt) (o0 GetResponse, o1 string, err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Get", []interface{}{i0, i1}, []interface{}{&o0, &o1}, opts...)
	return
}

// GroupReaderServerMethods is the interface a server writer
// implements for GroupReader.
//
// GroupReader implements methods to read or query a group's membership
// information.
type GroupReaderServerMethods interface {
	// Relate determines the relationships between the provided blessing
	// names and the members of the group.
	//
	// Given an input set of blessing names and a group defined by a set of
	// blessing patterns S, for each blessing name B in the input, Relate(B)
	// returns a set of "remainders" consisting of every blessing name B"
	// such that there exists some B' for which B = B' B" and B' is in S,
	// and "" if B is a member of S.
	//
	// For example, if a group is defined as S = {n1, n1:n2, n1:n2:n3}, then
	// Relate(n1:n2) = {n2, ""}.
	//
	// reqVersion specifies the expected version of the group's membership
	// information. If this version is set and matches the Group's current
	// version, the response will indicate that fact but will otherwise be
	// empty.
	//
	// visitedGroups is the set of groups already visited in a particular
	// chain of Relate calls, and is used to detect the presence of
	// cycles. When a cycle is detected, it is treated just like any other
	// error, and the result is approximated.
	//
	// Relate also returns information about all the errors encountered that
	// resulted in approximations, if any.
	//
	// TODO(hpucha): scrub "Approximation" for preserving privacy. Flesh
	// versioning out further. Other args we may need: option to Get() the
	// membership set when allowed (to avoid an extra RPC), options related
	// to caching this information.
	Relate(_ *context.T, _ rpc.ServerCall, blessings map[string]struct{}, hint ApproximationType, reqVersion string, visitedGroups map[string]struct{}) (remainder map[string]struct{}, approximations []Approximation, version string, _ error)
	// Get returns all entries in the group.
	// TODO(sadovsky): Flesh out this API.
	Get(_ *context.T, _ rpc.ServerCall, req GetRequest, reqVersion string) (res GetResponse, version string, _ error)
}

// GroupReaderServerStubMethods is the server interface containing
// GroupReader methods, as expected by rpc.Server.
// There is no difference between this interface and GroupReaderServerMethods
// since there are no streaming methods.
type GroupReaderServerStubMethods GroupReaderServerMethods

// GroupReaderServerStub adds universal methods to GroupReaderServerStubMethods.
type GroupReaderServerStub interface {
	GroupReaderServerStubMethods
	// Describe the GroupReader interfaces.
	Describe__() []rpc.InterfaceDesc
}

// GroupReaderServer returns a server stub for GroupReader.
// It converts an implementation of GroupReaderServerMethods into
// an object that may be used by rpc.Server.
func GroupReaderServer(impl GroupReaderServerMethods) GroupReaderServerStub {
	stub := implGroupReaderServerStub{
		impl: impl,
	}
	// Initialize GlobState; always check the stub itself first, to handle the
	// case where the user has the Glob method defined in their VDL source.
	if gs := rpc.NewGlobState(stub); gs != nil {
		stub.gs = gs
	} else if gs := rpc.NewGlobState(impl); gs != nil {
		stub.gs = gs
	}
	return stub
}

type implGroupReaderServerStub struct {
	impl GroupReaderServerMethods
	gs   *rpc.GlobState
}

func (s implGroupReaderServerStub) Relate(ctx *context.T, call rpc.ServerCall, i0 map[string]struct{}, i1 ApproximationType, i2 string, i3 map[string]struct{}) (map[string]struct{}, []Approximation, string, error) {
	return s.impl.Relate(ctx, call, i0, i1, i2, i3)
}

func (s implGroupReaderServerStub) Get(ctx *context.T, call rpc.ServerCall, i0 GetRequest, i1 string) (GetResponse, string, error) {
	return s.impl.Get(ctx, call, i0, i1)
}

func (s implGroupReaderServerStub) Globber() *rpc.GlobState {
	return s.gs
}

func (s implGroupReaderServerStub) Describe__() []rpc.InterfaceDesc {
	return []rpc.InterfaceDesc{GroupReaderDesc}
}

// GroupReaderDesc describes the GroupReader interface.
var GroupReaderDesc rpc.InterfaceDesc = descGroupReader

// descGroupReader hides the desc to keep godoc clean.
var descGroupReader = rpc.InterfaceDesc{
	Name:    "GroupReader",
	PkgPath: "v.io/v23/services/groups",
	Doc:     "// GroupReader implements methods to read or query a group's membership\n// information.",
	Methods: []rpc.MethodDesc{
		{
			Name: "Relate",
			Doc:  "// Relate determines the relationships between the provided blessing\n// names and the members of the group.\n//\n// Given an input set of blessing names and a group defined by a set of\n// blessing patterns S, for each blessing name B in the input, Relate(B)\n// returns a set of \"remainders\" consisting of every blessing name B\"\n// such that there exists some B' for which B = B' B\" and B' is in S,\n// and \"\" if B is a member of S.\n//\n// For example, if a group is defined as S = {n1, n1:n2, n1:n2:n3}, then\n// Relate(n1:n2) = {n2, \"\"}.\n//\n// reqVersion specifies the expected version of the group's membership\n// information. If this version is set and matches the Group's current\n// version, the response will indicate that fact but will otherwise be\n// empty.\n//\n// visitedGroups is the set of groups already visited in a particular\n// chain of Relate calls, and is used to detect the presence of\n// cycles. When a cycle is detected, it is treated just like any other\n// error, and the result is approximated.\n//\n// Relate also returns information about all the errors encountered that\n// resulted in approximations, if any.\n//\n// TODO(hpucha): scrub \"Approximation\" for preserving privacy. Flesh\n// versioning out further. Other args we may need: option to Get() the\n// membership set when allowed (to avoid an extra RPC), options related\n// to caching this information.",
			InArgs: []rpc.ArgDesc{
				{"blessings", ``},     // map[string]struct{}
				{"hint", ``},          // ApproximationType
				{"reqVersion", ``},    // string
				{"visitedGroups", ``}, // map[string]struct{}
			},
			OutArgs: []rpc.ArgDesc{
				{"remainder", ``},      // map[string]struct{}
				{"approximations", ``}, // []Approximation
				{"version", ``},        // string
			},
			Tags: []*vdl.Value{vdl.ValueOf(access.Tag("Resolve"))},
		},
		{
			Name: "Get",
			Doc:  "// Get returns all entries in the group.\n// TODO(sadovsky): Flesh out this API.",
			InArgs: []rpc.ArgDesc{
				{"req", ``},        // GetRequest
				{"reqVersion", ``}, // string
			},
			OutArgs: []rpc.ArgDesc{
				{"res", ``},     // GetResponse
				{"version", ``}, // string
			},
			Tags: []*vdl.Value{vdl.ValueOf(access.Tag("Read"))},
		},
	},
}

// GroupClientMethods is the client interface
// containing Group methods.
//
// A group's version covers its Permissions as well as any other data stored in
// the group. Clients should treat versions as opaque identifiers. For both Get
// and Relate, if version is set and matches the Group's current version, the
// response will indicate that fact but will otherwise be empty.
type GroupClientMethods interface {
	// GroupReader implements methods to read or query a group's membership
	// information.
	GroupReaderClientMethods
	// Object provides access control for Vanadium objects.
	//
	// Vanadium services implementing dynamic access control would typically embed
	// this interface and tag additional methods defined by the service with one of
	// Admin, Read, Write, Resolve etc. For example, the VDL definition of the
	// object would be:
	//
	//   package mypackage
	//
	//   import "v.io/v23/security/access"
	//   import "v.io/v23/services/permissions"
	//
	//   type MyObject interface {
	//     permissions.Object
	//     MyRead() (string, error) {access.Read}
	//     MyWrite(string) error    {access.Write}
	//   }
	//
	// If the set of pre-defined tags is insufficient, services may define their
	// own tag type and annotate all methods with this new type.
	//
	// Instead of embedding this Object interface, define SetPermissions and
	// GetPermissions in their own interface. Authorization policies will typically
	// respect annotations of a single type. For example, the VDL definition of an
	// object would be:
	//
	//  package mypackage
	//
	//  import "v.io/v23/security/access"
	//
	//  type MyTag string
	//
	//  const (
	//    Blue = MyTag("Blue")
	//    Red  = MyTag("Red")
	//  )
	//
	//  type MyObject interface {
	//    MyMethod() (string, error) {Blue}
	//
	//    // Allow clients to change access via the access.Object interface:
	//    SetPermissions(perms access.Permissions, version string) error         {Red}
	//    GetPermissions() (perms access.Permissions, version string, err error) {Blue}
	//  }
	permissions.ObjectClientMethods
	// Create creates a new group if it doesn't already exist.
	// If perms is nil, a default Permissions is used, providing Admin access to
	// the caller.
	// Create requires the caller to have Write permission at the GroupServer.
	Create(_ *context.T, perms access.Permissions, entries []BlessingPatternChunk, _ ...rpc.CallOpt) error
	// Delete deletes the group.
	// Permissions for all group-related methods except Create() are checked
	// against the Group object.
	Delete(_ *context.T, version string, _ ...rpc.CallOpt) error
	// Add adds an entry to the group.
	Add(_ *context.T, entry BlessingPatternChunk, version string, _ ...rpc.CallOpt) error
	// Remove removes an entry from the group.
	Remove(_ *context.T, entry BlessingPatternChunk, version string, _ ...rpc.CallOpt) error
}

// GroupClientStub adds universal methods to GroupClientMethods.
type GroupClientStub interface {
	GroupClientMethods
	rpc.UniversalServiceMethods
}

// GroupClient returns a client stub for Group.
func GroupClient(name string) GroupClientStub {
	return implGroupClientStub{name, GroupReaderClient(name), permissions.ObjectClient(name)}
}

type implGroupClientStub struct {
	name string

	GroupReaderClientStub
	permissions.ObjectClientStub
}

func (c implGroupClientStub) Create(ctx *context.T, i0 access.Permissions, i1 []BlessingPatternChunk, opts ...rpc.CallOpt) (err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Create", []interface{}{i0, i1}, nil, opts...)
	return
}

func (c implGroupClientStub) Delete(ctx *context.T, i0 string, opts ...rpc.CallOpt) (err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Delete", []interface{}{i0}, nil, opts...)
	return
}

func (c implGroupClientStub) Add(ctx *context.T, i0 BlessingPatternChunk, i1 string, opts ...rpc.CallOpt) (err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Add", []interface{}{i0, i1}, nil, opts...)
	return
}

func (c implGroupClientStub) Remove(ctx *context.T, i0 BlessingPatternChunk, i1 string, opts ...rpc.CallOpt) (err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Remove", []interface{}{i0, i1}, nil, opts...)
	return
}

// GroupServerMethods is the interface a server writer
// implements for Group.
//
// A group's version covers its Permissions as well as any other data stored in
// the group. Clients should treat versions as opaque identifiers. For both Get
// and Relate, if version is set and matches the Group's current version, the
// response will indicate that fact but will otherwise be empty.
type GroupServerMethods interface {
	// GroupReader implements methods to read or query a group's membership
	// information.
	GroupReaderServerMethods
	// Object provides access control for Vanadium objects.
	//
	// Vanadium services implementing dynamic access control would typically embed
	// this interface and tag additional methods defined by the service with one of
	// Admin, Read, Write, Resolve etc. For example, the VDL definition of the
	// object would be:
	//
	//   package mypackage
	//
	//   import "v.io/v23/security/access"
	//   import "v.io/v23/services/permissions"
	//
	//   type MyObject interface {
	//     permissions.Object
	//     MyRead() (string, error) {access.Read}
	//     MyWrite(string) error    {access.Write}
	//   }
	//
	// If the set of pre-defined tags is insufficient, services may define their
	// own tag type and annotate all methods with this new type.
	//
	// Instead of embedding this Object interface, define SetPermissions and
	// GetPermissions in their own interface. Authorization policies will typically
	// respect annotations of a single type. For example, the VDL definition of an
	// object would be:
	//
	//  package mypackage
	//
	//  import "v.io/v23/security/access"
	//
	//  type MyTag string
	//
	//  const (
	//    Blue = MyTag("Blue")
	//    Red  = MyTag("Red")
	//  )
	//
	//  type MyObject interface {
	//    MyMethod() (string, error) {Blue}
	//
	//    // Allow clients to change access via the access.Object interface:
	//    SetPermissions(perms access.Permissions, version string) error         {Red}
	//    GetPermissions() (perms access.Permissions, version string, err error) {Blue}
	//  }
	permissions.ObjectServerMethods
	// Create creates a new group if it doesn't already exist.
	// If perms is nil, a default Permissions is used, providing Admin access to
	// the caller.
	// Create requires the caller to have Write permission at the GroupServer.
	Create(_ *context.T, _ rpc.ServerCall, perms access.Permissions, entries []BlessingPatternChunk) error
	// Delete deletes the group.
	// Permissions for all group-related methods except Create() are checked
	// against the Group object.
	Delete(_ *context.T, _ rpc.ServerCall, version string) error
	// Add adds an entry to the group.
	Add(_ *context.T, _ rpc.ServerCall, entry BlessingPatternChunk, version string) error
	// Remove removes an entry from the group.
	Remove(_ *context.T, _ rpc.ServerCall, entry BlessingPatternChunk, version string) error
}

// GroupServerStubMethods is the server interface containing
// Group methods, as expected by rpc.Server.
// There is no difference between this interface and GroupServerMethods
// since there are no streaming methods.
type GroupServerStubMethods GroupServerMethods

// GroupServerStub adds universal methods to GroupServerStubMethods.
type GroupServerStub interface {
	GroupServerStubMethods
	// Describe the Group interfaces.
	Describe__() []rpc.InterfaceDesc
}

// GroupServer returns a server stub for Group.
// It converts an implementation of GroupServerMethods into
// an object that may be used by rpc.Server.
func GroupServer(impl GroupServerMethods) GroupServerStub {
	stub := implGroupServerStub{
		impl:                  impl,
		GroupReaderServerStub: GroupReaderServer(impl),
		ObjectServerStub:      permissions.ObjectServer(impl),
	}
	// Initialize GlobState; always check the stub itself first, to handle the
	// case where the user has the Glob method defined in their VDL source.
	if gs := rpc.NewGlobState(stub); gs != nil {
		stub.gs = gs
	} else if gs := rpc.NewGlobState(impl); gs != nil {
		stub.gs = gs
	}
	return stub
}

type implGroupServerStub struct {
	impl GroupServerMethods
	GroupReaderServerStub
	permissions.ObjectServerStub
	gs *rpc.GlobState
}

func (s implGroupServerStub) Create(ctx *context.T, call rpc.ServerCall, i0 access.Permissions, i1 []BlessingPatternChunk) error {
	return s.impl.Create(ctx, call, i0, i1)
}

func (s implGroupServerStub) Delete(ctx *context.T, call rpc.ServerCall, i0 string) error {
	return s.impl.Delete(ctx, call, i0)
}

func (s implGroupServerStub) Add(ctx *context.T, call rpc.ServerCall, i0 BlessingPatternChunk, i1 string) error {
	return s.impl.Add(ctx, call, i0, i1)
}

func (s implGroupServerStub) Remove(ctx *context.T, call rpc.ServerCall, i0 BlessingPatternChunk, i1 string) error {
	return s.impl.Remove(ctx, call, i0, i1)
}

func (s implGroupServerStub) Globber() *rpc.GlobState {
	return s.gs
}

func (s implGroupServerStub) Describe__() []rpc.InterfaceDesc {
	return []rpc.InterfaceDesc{GroupDesc, GroupReaderDesc, permissions.ObjectDesc}
}

// GroupDesc describes the Group interface.
var GroupDesc rpc.InterfaceDesc = descGroup

// descGroup hides the desc to keep godoc clean.
var descGroup = rpc.InterfaceDesc{
	Name:    "Group",
	PkgPath: "v.io/v23/services/groups",
	Doc:     "// A group's version covers its Permissions as well as any other data stored in\n// the group. Clients should treat versions as opaque identifiers. For both Get\n// and Relate, if version is set and matches the Group's current version, the\n// response will indicate that fact but will otherwise be empty.",
	Embeds: []rpc.EmbedDesc{
		{"GroupReader", "v.io/v23/services/groups", "// GroupReader implements methods to read or query a group's membership\n// information."},
		{"Object", "v.io/v23/services/permissions", "// Object provides access control for Vanadium objects.\n//\n// Vanadium services implementing dynamic access control would typically embed\n// this interface and tag additional methods defined by the service with one of\n// Admin, Read, Write, Resolve etc. For example, the VDL definition of the\n// object would be:\n//\n//   package mypackage\n//\n//   import \"v.io/v23/security/access\"\n//   import \"v.io/v23/services/permissions\"\n//\n//   type MyObject interface {\n//     permissions.Object\n//     MyRead() (string, error) {access.Read}\n//     MyWrite(string) error    {access.Write}\n//   }\n//\n// If the set of pre-defined tags is insufficient, services may define their\n// own tag type and annotate all methods with this new type.\n//\n// Instead of embedding this Object interface, define SetPermissions and\n// GetPermissions in their own interface. Authorization policies will typically\n// respect annotations of a single type. For example, the VDL definition of an\n// object would be:\n//\n//  package mypackage\n//\n//  import \"v.io/v23/security/access\"\n//\n//  type MyTag string\n//\n//  const (\n//    Blue = MyTag(\"Blue\")\n//    Red  = MyTag(\"Red\")\n//  )\n//\n//  type MyObject interface {\n//    MyMethod() (string, error) {Blue}\n//\n//    // Allow clients to change access via the access.Object interface:\n//    SetPermissions(perms access.Permissions, version string) error         {Red}\n//    GetPermissions() (perms access.Permissions, version string, err error) {Blue}\n//  }"},
	},
	Methods: []rpc.MethodDesc{
		{
			Name: "Create",
			Doc:  "// Create creates a new group if it doesn't already exist.\n// If perms is nil, a default Permissions is used, providing Admin access to\n// the caller.\n// Create requires the caller to have Write permission at the GroupServer.",
			InArgs: []rpc.ArgDesc{
				{"perms", ``},   // access.Permissions
				{"entries", ``}, // []BlessingPatternChunk
			},
			Tags: []*vdl.Value{vdl.ValueOf(access.Tag("Write"))},
		},
		{
			Name: "Delete",
			Doc:  "// Delete deletes the group.\n// Permissions for all group-related methods except Create() are checked\n// against the Group object.",
			InArgs: []rpc.ArgDesc{
				{"version", ``}, // string
			},
			Tags: []*vdl.Value{vdl.ValueOf(access.Tag("Write"))},
		},
		{
			Name: "Add",
			Doc:  "// Add adds an entry to the group.",
			InArgs: []rpc.ArgDesc{
				{"entry", ``},   // BlessingPatternChunk
				{"version", ``}, // string
			},
			Tags: []*vdl.Value{vdl.ValueOf(access.Tag("Write"))},
		},
		{
			Name: "Remove",
			Doc:  "// Remove removes an entry from the group.",
			InArgs: []rpc.ArgDesc{
				{"entry", ``},   // BlessingPatternChunk
				{"version", ``}, // string
			},
			Tags: []*vdl.Value{vdl.ValueOf(access.Tag("Write"))},
		},
	},
}

// Hold type definitions in package-level variables, for better performance.
// nolint: unused
var (
	__VDLType_string_1 *vdl.Type
	__VDLType_struct_2 *vdl.Type
	__VDLType_struct_3 *vdl.Type
	__VDLType_set_4    *vdl.Type
	__VDLType_enum_5   *vdl.Type
	__VDLType_struct_6 *vdl.Type
)

var __VDLInitCalled bool

// __VDLInit performs vdl initialization.  It is safe to call multiple times.
// If you have an init ordering issue, just insert the following line verbatim
// into your source files in this package, right after the "package foo" clause:
//
//    var _ = __VDLInit()
//
// The purpose of this function is to ensure that vdl initialization occurs in
// the right order, and very early in the init sequence.  In particular, vdl
// registration and package variable initialization needs to occur before
// functions like vdl.TypeOf will work properly.
//
// This function returns a dummy value, so that it can be used to initialize the
// first var in the file, to take advantage of Go's defined init order.
func __VDLInit() struct{} {
	if __VDLInitCalled {
		return struct{}{}
	}
	__VDLInitCalled = true

	// Register types.
	vdl.Register((*BlessingPatternChunk)(nil))
	vdl.Register((*GetRequest)(nil))
	vdl.Register((*GetResponse)(nil))
	vdl.Register((*ApproximationType)(nil))
	vdl.Register((*Approximation)(nil))

	// Initialize type definitions.
	__VDLType_string_1 = vdl.TypeOf((*BlessingPatternChunk)(nil))
	__VDLType_struct_2 = vdl.TypeOf((*GetRequest)(nil)).Elem()
	__VDLType_struct_3 = vdl.TypeOf((*GetResponse)(nil)).Elem()
	__VDLType_set_4 = vdl.TypeOf((*map[BlessingPatternChunk]struct{})(nil))
	__VDLType_enum_5 = vdl.TypeOf((*ApproximationType)(nil))
	__VDLType_struct_6 = vdl.TypeOf((*Approximation)(nil)).Elem()

	// Set error format strings.
	i18n.Cat().SetWithBase(i18n.LangID("en"), i18n.MsgID(ErrNoBlessings.ID), "{1:}{2:} No blessings recognized; cannot create group Permissions")
	i18n.Cat().SetWithBase(i18n.LangID("en"), i18n.MsgID(ErrExcessiveContention.ID), "{1:}{2:} Gave up after encountering excessive contention; try again later")
	i18n.Cat().SetWithBase(i18n.LangID("en"), i18n.MsgID(ErrCycleFound.ID), "{1:}{2:} Found cycle in group definitions{:_}")

	return struct{}{}
}

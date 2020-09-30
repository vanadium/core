// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import (
	"strings"

	"v.io/v23/context"
	"v.io/v23/glob"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/rpc/reserved"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/vdl"
	"v.io/v23/vdlroot/signature"
	"v.io/v23/verror"
)

// reservedInvoker returns a special invoker for reserved methods.  This invoker
// has access to the internal dispatchers, which allows it to perform special
// handling for methods like Glob and Signature.
func reservedInvoker(dispNormal, dispReserved rpc.Dispatcher) rpc.Invoker {
	methods := &reservedMethods{dispNormal: dispNormal, dispReserved: dispReserved}
	invoker := rpc.ReflectInvokerOrDie(methods)
	methods.selfInvoker = invoker
	return invoker
}

// reservedMethods is a regular server implementation object, which is passed to
// the regular ReflectInvoker in order to implement reserved methods.  The
// leading reserved "__" prefix is stripped before any methods are called.
//
// To add a new reserved method, simply add a method below, along with a
// description of the method.
type reservedMethods struct {
	dispNormal   rpc.Dispatcher
	dispReserved rpc.Dispatcher
	selfInvoker  rpc.Invoker
}

//nolint:golint // API change required.
func (r *reservedMethods) Describe__() []rpc.InterfaceDesc {
	return []rpc.InterfaceDesc{{
		Name: "__Reserved",
		Doc:  `Reserved methods implemented by the RPC framework.  Each method name is prefixed with a double underscore "__".`,
		Methods: []rpc.MethodDesc{
			{
				Name:      "Glob",
				Doc:       "Glob returns all entries matching the pattern.",
				InArgs:    []rpc.ArgDesc{{Name: "pattern", Doc: ""}},
				OutStream: rpc.ArgDesc{Doc: "Streams matching entries back to the client."},
			},
			{
				Name: "MethodSignature",
				Doc:  "MethodSignature returns the signature for the given method.",
				InArgs: []rpc.ArgDesc{{
					Name: "method",
					Doc:  "Method name whose signature will be returned.",
				}},
				OutArgs: []rpc.ArgDesc{{
					Doc: "Method signature for the given method.",
				}},
			},
			{
				Name: "Signature",
				Doc:  "Signature returns all interface signatures implemented by the object.",
				OutArgs: []rpc.ArgDesc{{
					Doc: "All interface signatures implemented by the object.",
				}},
			},
		},
	}}
}

func (r *reservedMethods) Signature(ctx *context.T, call rpc.ServerCall) ([]signature.Interface, error) {
	suffix := call.Suffix()
	disp := r.dispNormal
	if naming.IsReserved(suffix) {
		disp = r.dispReserved
	}
	if disp == nil {
		return nil, verror.ErrUnknownSuffix.Errorf(ctx, "suffix does not exist: %v", suffix)
	}
	obj, _, err := disp.Lookup(ctx, suffix)
	switch {
	case err != nil:
		return nil, err
	case obj == nil:
		return nil, verror.ErrUnknownSuffix.Errorf(ctx, "suffix does not exist: %v", suffix)
	}
	invoker, err := objectToInvoker(obj)
	if err != nil {
		return nil, err
	}
	sig, err := invoker.Signature(ctx, call)
	if err != nil {
		return nil, err
	}
	// Append the reserved methods.  We wait until now to add the "__" prefix to
	// each method, so that we can use the regular ReflectInvoker.Signature logic.
	rsig, err := r.selfInvoker.Signature(ctx, call)
	if err != nil {
		return nil, err
	}
	for i := range rsig {
		for j := range rsig[i].Methods {
			rsig[i].Methods[j].Name = "__" + rsig[i].Methods[j].Name
		}
	}
	return signature.CleanInterfaces(append(sig, rsig...)), nil
}

func (r *reservedMethods) MethodSignature(ctx *context.T, call rpc.ServerCall, method string) (signature.Method, error) {
	// Reserved methods use our self invoker, to describe our own methods,
	if naming.IsReserved(method) {
		return r.selfInvoker.MethodSignature(ctx, call, naming.StripReserved(method))
	}

	suffix := call.Suffix()
	disp := r.dispNormal
	if naming.IsReserved(suffix) {
		disp = r.dispReserved
	}
	if disp == nil {
		return signature.Method{}, verror.ErrUnknownMethod.Errorf(ctx, "method does not exist: %v", rpc.ReservedMethodSignature)
	}
	obj, _, err := disp.Lookup(ctx, suffix)
	switch {
	case err != nil:
		return signature.Method{}, err
	case obj == nil:
		return signature.Method{}, verror.ErrUnknownMethod.Errorf(ctx, "method does not exist: %v", rpc.ReservedMethodSignature)
	}
	invoker, err := objectToInvoker(obj)
	if err != nil {
		return signature.Method{}, err
	}
	// TODO(toddw): Decide if we should hide the method signature if the
	// caller doesn't have access to call it.
	return invoker.MethodSignature(ctx, call, method)
}

func (r *reservedMethods) Glob(ctx *context.T, call rpc.StreamServerCall, pattern string) error {
	// Copy the original call to shield ourselves from changes the flowServer makes.
	glob := globInternal{r.dispNormal, r.dispReserved, call.Suffix()}
	return glob.Glob(ctx, call, pattern)
}

// globInternal handles ALL the Glob requests received by a server and
// constructs a response from the state of internal server objects and the
// service objects.
//
// Internal objects exist only at the root of the server and have a name that
// starts with a double underscore ("__"). They are only visible in the Glob
// response if the double underscore is explicitly part of the pattern, e.g.
// "".Glob("__*/*"), or "".Glob("__debug/...").
//
// Service objects may choose to implement either AllGlobber or ChildrenGlobber.
// AllGlobber is more flexible, but ChildrenGlobber is simpler to implement and
// less prone to errors.
//
// If objects implement AllGlobber, it must be able to handle recursive pattern
// for the entire namespace below the receiver object, i.e. "a/b".Glob("...")
// must return the name of all the objects under "a/b".
//
// If they implement ChildrenGlobber, it provides a list of the receiver's
// immediate children names, or a non-nil error if the receiver doesn't exist.
//
// globInternal constructs the Glob response by internally accessing the
// AllGlobber or ChildrenGlobber interface of objects as many times as needed.
//
// Before accessing an object, globInternal ensures that the requester is
// authorized to access it. Internal objects require access.Debug. Service
// objects require access.Resolve.
type globInternal struct {
	dispNormal   rpc.Dispatcher
	dispReserved rpc.Dispatcher
	receiver     string
}

// The maximum depth of recursion in Glob. We only count recursion levels
// associated with a recursive glob pattern, e.g. a pattern like "..." will be
// allowed to recurse up to 10 levels, but "*/*/*/*/..." will go up to 14
// levels.
const maxRecursiveGlobDepth = 10

type gState struct {
	name  string
	glob  *glob.Glob
	depth int
}

func (i *globInternal) Glob(ctx *context.T, call rpc.StreamServerCall, pattern string) error { //nolint:gocyclo
	ctx.VI(3).Infof("rpc Glob: Incoming request: %q.Glob(%q)", i.receiver, pattern)
	g, err := glob.Parse(pattern)
	if err != nil {
		return err
	}
	disp := i.dispNormal
	tags := []*vdl.Value{vdl.ValueOf(access.Resolve)}
	if naming.IsReserved(i.receiver) || (i.receiver == "" && naming.IsReserved(pattern)) {
		disp = i.dispReserved
		tags = []*vdl.Value{vdl.ValueOf(access.Debug)}
	}
	if disp == nil {
		return reserved.ErrGlobNotImplemented.Errorf(ctx, "glob not implemented")
	}
	call = callWithMethodTags(ctx, call, tags)

	queue := []gState{{glob: g}}

	someMatchesOmitted := false
	for len(queue) != 0 {
		select {
		case <-ctx.Done():
			// RPC timed out or was canceled.
			return nil
		default:
		}
		state := queue[0]
		queue = queue[1:]

		subcall := callWithSuffix(ctx, call, naming.Join(i.receiver, state.name))
		suffix := subcall.Suffix()
		if state.depth > maxRecursiveGlobDepth {
			ctx.Errorf("rpc Glob: exceeded recursion limit (%d): %q", maxRecursiveGlobDepth, suffix)
			//nolint:errcheck
			subcall.Send(naming.GlobReplyError{
				Value: naming.GlobError{Name: state.name, Error: reserved.ErrGlobMaxRecursionReached.Errorf(ctx, "max recursion level reached")},
			})
			continue
		}
		obj, auth, err := disp.Lookup(ctx, suffix)
		if err != nil {
			ctx.VI(3).Infof("rpc Glob: Lookup failed for %q: %v", suffix, err)
			//nolint:errcheck
			subcall.Send(naming.GlobReplyError{
				Value: naming.GlobError{Name: state.name, Error: verror.Convert(verror.ErrNoExist, ctx, err)},
			})
			continue
		}
		if obj == nil {
			ctx.VI(3).Infof("rpc Glob: object not found for %q", suffix)
			//nolint:errcheck
			subcall.Send(naming.GlobReplyError{
				Value: naming.GlobError{
					Name:  state.name,
					Error: verror.ErrNoExist.Errorf(ctx, "does not exist: nil object")},
			})
			continue
		}

		// Verify that that requester is authorized for the current object.
		if err := authorize(ctx, subcall.Security(), auth); err != nil {
			someMatchesOmitted = true
			ctx.VI(3).Infof("rpc Glob: client is not authorized for %q: %v", suffix, err)
			continue
		}

		// If the object implements both AllGlobber and ChildrenGlobber, we'll
		// use AllGlobber.
		invoker, err := objectToInvoker(obj)
		if err != nil {
			ctx.VI(3).Infof("rpc Glob: object for %q cannot be converted to invoker: %v", suffix, err)
			//nolint:errcheck
			subcall.Send(naming.GlobReplyError{
				Value: naming.GlobError{Name: state.name, Error: verror.Convert(verror.ErrInternal, ctx, err)},
			})
			continue
		}
		gs := invoker.Globber()
		if gs == nil || (gs.AllGlobber == nil && gs.ChildrenGlobber == nil) {
			if state.glob.Len() == 0 {
				//nolint:errcheck
				subcall.Send(naming.GlobReplyEntry{
					Value: naming.MountEntry{Name: state.name, IsLeaf: true},
				})
			} else {
				//nolint:errcheck
				subcall.Send(naming.GlobReplyError{
					Value: naming.GlobError{
						Name:  state.name,
						Error: reserved.ErrGlobNotImplemented.Errorf(ctx, "glob not implemented")},
				})
			}
			continue
		}
		if gs.AllGlobber != nil {
			ctx.VI(3).Infof("rpc Glob: %q implements AllGlobber", suffix)
			send := func(reply naming.GlobReply) error {
				select {
				case <-ctx.Done():
					return verror.ErrAborted.Errorf(ctx, "aborted")
				default:
				}
				switch v := reply.(type) {
				case naming.GlobReplyEntry:
					v.Value.Name = naming.Join(state.name, v.Value.Name)
					return subcall.Send(v)
				case naming.GlobReplyError:
					v.Value.Name = naming.Join(state.name, v.Value.Name)
					return subcall.Send(v)
				}
				return nil
			}
			if err := gs.AllGlobber.Glob__(ctx, &globServerCall{subcall, send}, state.glob); err != nil {
				ctx.VI(3).Infof("rpc Glob: %q.Glob(%q) failed: %v", suffix, state.glob, err)
				//nolint:errcheck
				subcall.Send(naming.GlobReplyError{
					Value: naming.GlobError{Name: state.name, Error: verror.Convert(verror.ErrInternal, ctx, err)},
				})
			}
			continue
		}
		if gs.ChildrenGlobber != nil {
			ctx.VI(3).Infof("rpc Glob: %q implements ChildrenGlobber", suffix)
			depth := state.depth
			if state.glob.Len() == 0 {
				// The glob pattern matches the current object.
				//nolint:errcheck
				subcall.Send(naming.GlobReplyEntry{
					Value: naming.MountEntry{Name: state.name},
				})
				if state.glob.Recursive() {
					// This is a recursive pattern. Make sure we don't recurse forever.
					depth++
				} else {
					// The pattern can't possibly match any children of this node.
					continue
				}
			}
			matcher, tail := state.glob.Head(), state.glob.Tail()
			send := func(reply naming.GlobChildrenReply) error {
				select {
				case <-ctx.Done():
					return verror.ErrAborted.Errorf(ctx, "aborted")
				default:
				}
				switch v := reply.(type) {
				case naming.GlobChildrenReplyName:
					child := v.Value
					if len(child) == 0 || strings.Contains(child, "/") {
						return verror.ErrBadArg.Errorf(ctx, "bad argument: invalid child name: %q", child)
					}
					if !matcher.Match(child) {
						return verror.ErrBadArg.Errorf(ctx, "bad argument: child name does not match: %q", child)
					}
					next := naming.Join(state.name, child)
					queue = append(queue, gState{next, tail, depth})
				case naming.GlobChildrenReplyError:
					v.Value.Name = naming.Join(state.name, v.Value.Name)
					//nolint:errcheck
					return subcall.Send(naming.GlobReplyError(v))
				}
				return nil
			}
			if err := gs.ChildrenGlobber.GlobChildren__(ctx, &globChildrenServerCall{subcall, send}, matcher); err != nil {
				//nolint:errcheck
				subcall.Send(naming.GlobReplyError{
					Value: naming.GlobError{Name: state.name, Error: verror.Convert(verror.ErrInternal, ctx, err)},
				})
			}
			continue
		}
	}
	if someMatchesOmitted {
		//nolint:errcheck
		call.Send(naming.GlobReplyError{
			Value: naming.GlobError{Error: reserved.ErrGlobMatchesOmitted.Errorf(ctx, "some matches might have been omitted")},
		})
	}
	return nil
}

type globServerCall struct {
	rpc.ServerCall
	send func(reply naming.GlobReply) error
}

func (g *globServerCall) SendStream() interface {
	Send(naming.GlobReply) error
} {
	return g
}

func (g *globServerCall) Send(reply naming.GlobReply) error {
	return g.send(reply)
}

type globChildrenServerCall struct {
	rpc.ServerCall
	send func(reply naming.GlobChildrenReply) error
}

func (g *globChildrenServerCall) SendStream() interface {
	Send(naming.GlobChildrenReply) error
} {
	return g
}

func (g *globChildrenServerCall) Send(reply naming.GlobChildrenReply) error {
	return g.send(reply)
}

// derivedServerCall allows us to derive calls with slightly different properties,
// useful for our various special-cased reserved methods.
type derivedServerCall struct {
	rpc.StreamServerCall
	suffix   string
	security security.Call
}

func callWithSuffix(ctx *context.T, src rpc.StreamServerCall, suffix string) rpc.StreamServerCall {
	sec := securityCallWithSuffix(src.Security(), suffix)
	return &derivedServerCall{src, suffix, sec}
}

func callWithMethodTags(ctx *context.T, src rpc.StreamServerCall, tags []*vdl.Value) rpc.StreamServerCall {
	sec := securityCallWithMethodTags(src.Security(), tags)
	return &derivedServerCall{src, src.Suffix(), sec}
}

func (c *derivedServerCall) Suffix() string {
	return c.suffix
}
func (c *derivedServerCall) Security() security.Call {
	return c.security
}

type derivedSecurityCall struct {
	security.Call
	suffix     string
	methodTags []*vdl.Value
}

func securityCallWithSuffix(src security.Call, suffix string) security.Call {
	return &derivedSecurityCall{src, suffix, src.MethodTags()}
}

func securityCallWithMethodTags(src security.Call, tags []*vdl.Value) security.Call {
	return &derivedSecurityCall{src, src.Suffix(), tags}
}

func (c *derivedSecurityCall) Suffix() string {
	return c.suffix
}
func (c *derivedSecurityCall) MethodTags() []*vdl.Value {
	return c.methodTags
}

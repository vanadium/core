// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package verror implements an error reporting mechanism that works across
// programming environments, and a set of common errors. It captures the location
// and parameters of the error call site to aid debugging. Errorf is not intended
// for localization, whereas Message accepts a preformatted message to allow
// for localization via an alternative package/framework.
//
// Each error has an identifier string, which is used for equality checks.
// E.g. a Javascript client can check if a Go server returned a NoExist error by
// checking the string identifier.  Error identifier strings start with the VDL
// package path to ensure uniqueness, e.g. "v.io/v23/verror.NoExist".
// The NewID and NewIDAction functions automatically prepend the package path
// of the caller to the specified ID if it is not already included.
//
// Each error contains an action, which is the suggested action for a typical
// client to perform upon receiving the error.  E.g. some action codes represent
// whether to retry the operation after receiving the error.
//
// Each error also contains a list of typed parameters, and an error message.
// The error message may be created in two ways:
//   1. Via the Errorf method using fmt.Sprintf formatting.
//   2. Via the Message method where the error message is preformatted and the
//      parameter list is recorded.
//
// Example:
//
// To define a new error identifier, for example "someNewError", the code that
// originates the error is expected to declare a variable like this:
//
//     var someNewError = verror.Register("someNewError", NoRetry)
//     ...
//     return someNewError.Errorf(ctx, "my error message: %v", err)
//
// Alternatively, to use golang.org/x/text/messsage for localization:
//    p := message.NewPrinter(language.BritishEnglish)
//    msg := p.Sprintf("invalid name: %v: %v", name, err)
//    return someNewError.Message(ctx, msg, name, err)
//
//
// The verror implementation supports errors.Is and errors.Unwrap. Note
// that errors.Unwrap provides access to 'sub-errors' as well as to chained
// instances of error. verror.WithSubErrors can be used to add additional
// 'sub-errors' to an existing error and these may be of type SubErr or any
// other error.
//
// If the context.T specified with Errorf() or Message() is nil, a default
// context is used, set by SetDefaultContext().  This can be used in standalone
// programmes, or in anciliary threads not associated with an RPC.  The user
// might do the following to get the language from the environment, and the
// programme name from Args[0]:
//     ctx := runtime.NewContext()
//     ctx = verror.WithComponentName(ctx, os.Args[0])
//     verror.SetDefaultContext(ctx)
// A standalone tool might set the operation name to be a subcommand name, if
// any.  If the default context has not been set, the error generated has no
// component and operation values.
package verror

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"v.io/v23/context"
	"v.io/v23/vdl"
	"v.io/v23/vtrace"
)

// ID is a unique identifier for errors.
type ID string

// An ActionCode represents the action expected to be performed by a typical client
// receiving an error that perhaps it does not understand.
type ActionCode uint32

// Codes for ActionCode.
const (
	// Retry actions are encoded in the bottom few bits.
	RetryActionMask ActionCode = 3

	NoRetry         ActionCode = 0 // Do not retry.
	RetryConnection ActionCode = 1 // Renew high-level connection/context.
	RetryRefetch    ActionCode = 2 // Refetch and retry (e.g., out of date HTTP ETag)
	RetryBackoff    ActionCode = 3 // Backoff and retry a finite number of times.
)

// RetryAction returns the part of the ActionCode that indicates retry behaviour.
func (ac ActionCode) RetryAction() ActionCode {
	return ac & RetryActionMask
}

// An IDAction combines a unique identifier ID for errors with an ActionCode.
// The ID allows stable error checking across different error messages and
// different address spaces.  By convention the format for the identifier is
// "PKGPATH.NAME" - e.g. ErrIDFoo defined in the "v23/verror" package has id
// "v23/verror.ErrIDFoo".  It is unwise ever to create two IDActions that
// associate different ActionCodes with the same ID.
// IDAction implements error so that it may be used with errors.Is.
type IDAction struct {
	ID     ID
	Action ActionCode
}

// NewIDAction creates a new instance of IDAction with the given ID and Action
// field. It should be used when localization support is not required instead
// of Register. IDPath can be used to
func NewIDAction(id ID, action ActionCode) IDAction {
	return IDAction{ensurePackagePath(id), action}
}

// NewID creates a new instance of IDAction with the given ID and a NoRetry
// Action.
func NewID(id ID) IDAction {
	return IDAction{ensurePackagePath(id), NoRetry}
}

// Errorf creates a new verror.E that uses fmt.Errorf style formatting and is
// not intended for localization. Errorf prepends the component and operation
// name if they can be extracted from the context. It supports %w for errors.Unwrap
// which takes precedence over using the last parameter if it's an error as
// the error to be returned by Unwrap.
func (id IDAction) Errorf(ctx *context.T, format string, params ...interface{}) error {
	// handle %w.
	unwrap := errors.Unwrap(fmt.Errorf(format, params...))
	if unwrap == nil {
		unwrap = isLastParStandardError(params)
	}
	return verrorf(ctx, id, fmt.Sprintf(format, params...), unwrap, params)
}

// Message is intended for pre-internationalizated messages. The msg is assumed
// to be have been preformated and the params are recorded in E.ParamList. If
// the last parameter is an error it will returned by Unwrap.
func (id IDAction) Message(ctx *context.T, msg string, params ...interface{}) error {
	return verrorf(ctx, id, msg, isLastParStandardError(params), params)
}

// Errorf is like ErrUnknown.Errorf.
func Errorf(ctx *context.T, format string, params ...interface{}) error {
	// handle %w.
	unwrap := errors.Unwrap(fmt.Errorf(format, params...))
	if unwrap == nil {
		unwrap = isLastParStandardError(params)
	}
	return verrorf(ctx, ErrUnknown, fmt.Sprintf(format, params...), unwrap, params)
}

// Message is like ErrUnknown.Message.
func Message(ctx *context.T, msg string, params ...interface{}) error {
	return verrorf(ctx, ErrUnknown, msg, isLastParStandardError(params), params)
}

// IsAny returns true if err is any instance of a verror.E regardless of its
// ID.
func IsAny(err error) bool {
	if _, ok := err.(E); ok {
		return ok
	}
	_, ok := err.(*E)
	return ok
}

func isLastParStandardError(params []interface{}) error {
	if len(params) == 0 {
		return nil
	}
	c := params[len(params)-1]
	switch err := c.(type) {
	case SubErr:
		return nil
	case *SubErr:
		return nil
	case error:
		return err
	}
	return nil
}

func verrorf(ctx *context.T, id IDAction, msg string, unwrap error, v []interface{}) error {
	componentName, opName := dataFromContext(ctx)
	prefix := ""
	if len(componentName) > 0 && len(opName) > 0 {
		prefix += componentName + ":" + opName + ": "
	} else {
		if len(componentName) > 0 {
			prefix += componentName + ": "
		} else {
			prefix += opName + ": "
		}
	}
	stack := make([]uintptr, maxPCs)
	stack = stack[:runtime.Callers(3, stack)]
	chainedPCs := Stack(unwrap)
	params := append([]interface{}{componentName, opName}, v...)
	return E{id.ID, id.Action, prefix + msg, params, stack, chainedPCs, unwrap}
}

// WithSubErrors returns a new E with the supplied suberrors appended to
// its parameter list. The results of their Error method are appended to that
// of err.Error().
func WithSubErrors(err error, errors ...error) error {
	e, ok := assertIsE(err)
	if !ok {
		return err
	}
	for _, err := range errors {
		e.ParamList = append(e.ParamList, err)
		switch v := err.(type) {
		case SubErr:
			if v.Options == Print {
				e.Msg += " " + err.Error()
			}
		case *SubErr:
			if v.Options == Print {
				e.Msg += " " + err.Error()
			}
		case error:
			e.Msg += " " + err.Error()
		}
	}
	return e
}

// E is the in-memory representation of a verror error.
//
// The wire representation is defined as vdl.WireError; values of E
// type are automatically converted to/from vdl.WireError by VDL and VOM.
type E struct {
	ID         ID            // The identity of the error.
	Action     ActionCode    // Default action to take on error.
	Msg        string        // Error message; empty if no language known.
	ParamList  []interface{} // The variadic parameters given to ExplicitNew().
	stackPCs   []uintptr     // PCs of creators of E
	chainedPCs []uintptr     // PCs of a chained E
	unwrap     error         // The error to be returned by calls to Unwrap.
}

// TypeOf(verror.E{}) should give vdl.WireError.
func (E) VDLReflect(struct {
	Name string `vdl:"v.io/v23/vdl.WireError"`
}) {
}

func (e E) VDLEqual(yiface interface{}) bool {
	y := yiface.(E)
	switch {
	case e.ID != y.ID:
		return false
	case e.Action != y.Action:
		return false
	case e.Error() != y.Error():
		// NOTE: We compare the result of Error() rather than comparing the Msg
		// fields, since the Msg field isn't always set; Msg=="" is equivalent to
		// Msg==v.io/v23/verror.Unknown.
		//
		// TODO(toddw): Investigate the root cause of this.
		return false
	case len(e.ParamList) != len(y.ParamList):
		return false
	}
	for i := range e.ParamList {
		if !vdl.DeepEqual(e.ParamList[i], y.ParamList[i]) {
			return false
		}
	}
	return true
}

var (
	ttErrorElem     = vdl.ErrorType.Elem()
	ttWireRetryCode = ttErrorElem.Field(1).Type
	ttListAny       = vdl.ListType(vdl.AnyType)
)

func (e E) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.StartValue(ttErrorElem); err != nil {
		return err
	}
	if e.ID != "" {
		if err := enc.NextFieldValueString(0, vdl.StringType, string(e.ID)); err != nil {
			return err
		}
	}
	if e.Action != NoRetry {
		var actionStr string
		switch e.Action {
		case RetryConnection:
			actionStr = "RetryConnection"
		case RetryRefetch:
			actionStr = "RetryRefetch"
		case RetryBackoff:
			actionStr = "RetryBackoff"
		default:
			return fmt.Errorf("action %d not in enum WireRetryCode", e.Action)
		}
		if err := enc.NextFieldValueString(1, ttWireRetryCode, actionStr); err != nil {
			return err
		}
	}
	if e.Msg != "" {
		if err := enc.NextFieldValueString(2, vdl.StringType, e.Msg); err != nil {
			return err
		}
	}
	if len(e.ParamList) != 0 {
		if err := enc.NextField(3); err != nil {
			return err
		}
		if err := enc.StartValue(ttListAny); err != nil {
			return err
		}
		if err := enc.SetLenHint(len(e.ParamList)); err != nil {
			return err
		}
		for i := 0; i < len(e.ParamList); i++ {
			if err := enc.NextEntry(false); err != nil {
				return err
			}
			if e.ParamList[i] == nil {
				if err := enc.NilValue(vdl.AnyType); err != nil {
					return err
				}
			} else {
				if err := vdl.Write(enc, e.ParamList[i]); err != nil {
					return err
				}
			}
		}
		if err := enc.NextEntry(true); err != nil {
			return err
		}
		if err := enc.FinishValue(); err != nil {
			return err
		}
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (e *E) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	*e = E{}
	if err := dec.StartValue(ttErrorElem); err != nil {
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
		if decType != ttErrorElem {
			index = ttErrorElem.FieldIndexByName(decType.Field(index).Name)
			if index == -1 {
				if err := dec.SkipValue(); err != nil {
					return err
				}
				continue
			}
		}
	errorFieldSwitch:
		switch index {
		case 0:
			id, err := dec.ReadValueString()
			if err != nil {
				return err
			}
			e.ID = ID(id)
		case 1:
			code, err := dec.ReadValueString()
			if err != nil {
				return err
			}
			switch code {
			case "NoRetry":
				e.Action = NoRetry
			case "RetryConnection":
				e.Action = RetryConnection
			case "RetryRefetch":
				e.Action = RetryRefetch
			case "RetryBackoff":
				e.Action = RetryBackoff
			default:
				return fmt.Errorf("label %s not in enum WireRetryCode", code)
			}
		case 2:
			msg, err := dec.ReadValueString()
			if err != nil {
				return err
			}
			e.Msg = msg
		case 3:
			if err := dec.StartValue(ttListAny); err != nil {
				return err
			}
			switch len := dec.LenHint(); {
			case len > 0:
				e.ParamList = make([]interface{}, 0, len)
			default:
				e.ParamList = nil
			}
			for {
				switch done, err := dec.NextEntry(); {
				case err != nil:
					return err
				case done:
					if err := dec.FinishValue(); err != nil {
						return err
					}
					break errorFieldSwitch
				}
				var elem interface{}
				if err := vdl.Read(dec, &elem); err != nil {
					return err
				}
				e.ParamList = append(e.ParamList, elem)
			}
		}
	}
}

const maxPCs = 40 // Maximum number of PC values we'll include in a stack trace.

type SubErrOpts uint32

// A SubErr represents a (string, error, int32) triple that allows clients to
// include subordinate errors to an error's parameter list.  Clients can add a
// SubErr to the parameter list directly, via WithSubErrors. By convention,
// clients are expected to use name of the form "X=Y" to distinguish their
// subordinate errors from those of other abstraction layers.
// For example, a layer reporting on errors in individual blessings in an RPC
// might use strings like "blessing=<blessing_name>".
type SubErr struct {
	Name    string
	Err     error
	Options SubErrOpts
}

const (
	// Print, when set in SubErr.Options, tells Error() to print this SubErr.
	Print SubErrOpts = 0x1
)

func assertIsE(err error) (E, bool) {
	switch e := err.(type) {
	case E:
		return e, true
	case *E:
		if e != nil {
			return *e, true
		}
	}
	return E{}, false
}

// ErrorID returns the ID of the given err, or Unknown if the err has no ID.
// If err is nil then ErrorID returns "".
func ErrorID(err error) ID {
	if err == nil {
		return ""
	}
	if e, ok := assertIsE(err); ok {
		return e.ID
	}
	return ErrUnknown.ID
}

// Params returns the ParameterList stored in err if it's an instance
// of verror.E, nil otherwise.
func Params(err error) []interface{} {
	verr, ok := assertIsE(err)
	if !ok {
		return nil
	}
	cp := make([]interface{}, len(verr.ParamList))
	copy(cp, verr.ParamList)
	return cp
}

// Action returns the action of the given err, or NoRetry if the err has no Action.
func Action(err error) ActionCode {
	if err == nil {
		return NoRetry
	}
	if e, ok := assertIsE(err); ok {
		return e.Action
	}
	return NoRetry
}

// PCs represents a list of PC locations
type PCs []uintptr

// Stack returns the list of PC locations on the stack when this error was
// first generated within this address space, or an empty list if err is not an
// E.
func Stack(err error) PCs {
	if err == nil {
		return nil
	}
	switch e := err.(type) {
	case E:
		return stack(e.stackPCs, e.chainedPCs)
	case SubErr:
		return Stack(e.Err)
	case subErrChain:
		return Stack(e.err)
	case *E:
		return stack(e.stackPCs, e.chainedPCs)
	case *SubErr:
		return Stack(e.Err)
	case *subErrChain:
		return Stack(e.err)
	}
	return nil
}

func stack(stackPCs, chainedPCs []uintptr) []uintptr {
	stackIntPtr := make([]uintptr, len(stackPCs))
	copy(stackIntPtr, stackPCs)
	if chainedPCs != nil {
		stackIntPtr = append(stackIntPtr, uintptr(0))
		stackIntPtr = append(stackIntPtr, chainedPCs...)
	}
	return stackIntPtr
}

func (st PCs) String() string {
	buf := bytes.NewBufferString("")
	StackToText(buf, st)
	return buf.String()
}

func writeFrames(w io.Writer, indent string, stack []uintptr) {
	frames := runtime.CallersFrames(stack)
	for {
		frame, more := frames.Next()
		fmt.Fprintf(w, "%s%s:%d: %s\n", indent, frame.File, frame.Line, frame.Function)
		if !more {
			return
		}
	}
}

// stackToTextIndent emits on w a text representation of stack, which is typically
// obtained from Stack() and represents the source location(s) where an
// error was generated or passed through in the local address space.
// indent is added to a prefix of each line printed.
func stackToTextIndent(w io.Writer, stack []uintptr, indent string) {
	prev := 0
	for i := 0; i < len(stack); i++ {
		if stack[i] == 0 {
			writeFrames(w, indent, stack[prev:i])
			fmt.Fprintf(w, "%s----- chained verror -----\n", indent)
			prev = i + 1
		}
	}
	if prev < len(stack) {
		writeFrames(w, indent, stack[prev:])
	}
}

// StackToText emits on w a text representation of stack, which is typically
// obtained from Stack() and represents the source location(s) where an
// error was generated or passed through in the local address space.
func StackToText(w io.Writer, stack []uintptr) {
	stackToTextIndent(w, stack, "")
}

// A componentKey is used as a key for context.T's Value() map.
type componentKey struct{}

// WithComponentName returns a context based on ctx that has the
// componentName that New() and Convert() can use.
func WithComponentName(ctx *context.T, componentName string) *context.T {
	return context.WithValue(ctx, componentKey{}, componentName)
}

// defaultCtx is the context used when a nil context.T is passed to New() or Convert().
var (
	defaultCtx     *context.T
	defaultCtxLock sync.RWMutex // Protects defaultCtx.
)

// SetDefaultContext sets the default context used when a nil context.T is
// passed to New() or Convert().  It is typically used in standalone
// programmes that have no RPC context, or in servers for the context of
// ancillary threads not associated with any particular RPC.
func SetDefaultContext(ctx *context.T) {
	defaultCtxLock.Lock()
	defaultCtx = ctx
	defaultCtxLock.Unlock()
}

// dataFromContext reads the languageID, component name, and operation name
// from the context, using defaults as appropriate.
func dataFromContext(ctx *context.T) (componentName string, opName string) {
	// Use a default context if ctx is nil.  defaultCtx may also be nil, so
	// further nil checks are required below.
	if ctx == nil {
		defaultCtxLock.RLock()
		ctx = defaultCtx
		defaultCtxLock.RUnlock()
	}
	if ctx != nil {
		value := ctx.Value(componentKey{})
		componentName, _ = value.(string)
		opName = vtrace.GetSpan(ctx).Name()
	}
	if componentName == "" {
		componentName = filepath.Base(os.Args[0])
	}
	return componentName, opName
}

// Error returns the error message; if it has not been formatted for a specific
// language, a default message containing the error ID and parameters is
// generated.  This method is required to fulfil the error interface.
func (e E) Error() string {
	return e.Msg
}

func (subErr SubErr) String() string {
	return fmt.Sprintf("[%s: %s]", subErr.Name, subErr.Err.Error())
}

// debugStringInternal returns a more verbose string representation of an
// error, perhaps more thorough than one might present to an end user, but
// useful for debugging by a developer.  It prefixes all lines output with
// "prefix" and "name" (if non-empty) and adds intent to prefix wen recursing.
func debugStringInternal(out *strings.Builder, err error, prefix string, name string) {
	out.WriteString(prefix)
	if len(name) > 0 {
		out.WriteString(name + " ")
	}
	out.WriteString(err.Error())
	out.WriteString("\n")
	// Append err's stack, indented a little.
	prefix += "  "
	stackToTextIndent(out, Stack(err), prefix) //nolint:errcheck
	// Print all the subordinate errors, even the ones that were not
	// printed by Error(), indented a bit further.
	prefix += "  "
	i := 1
	for subErr := errors.Unwrap(err); subErr != nil; subErr = errors.Unwrap(subErr) {
		fmt.Fprintf(out, "%sUnwrapped error #%v\n", prefix, i)
		i++
		switch v := subErr.(type) {
		case subErrChain:
			debugStringInternal(out, v.err, prefix, string(ErrorID(v.err)))
		case SubErr:
			debugStringInternal(out, v.Err, prefix, v.Name)
		case *SubErr:
			debugStringInternal(out, v.Err, prefix, v.Name)
		case E:
			debugStringInternal(out, v, prefix, string(v.ID))
		case *E:
			debugStringInternal(out, v, prefix, string(v.ID))
		}
	}
}

// DebugString returns a more verbose string representation of an error,
// perhaps more thorough than one might present to an end user, but useful for
// debugging by a developer.
func DebugString(err error) string {
	if err == nil {
		return ""
	}
	out := &strings.Builder{}
	debugStringInternal(out, err, "", "")
	return out.String()
}

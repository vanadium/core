// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pproflib

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"v.io/v23/context"
	"v.io/v23/rpc"
	s_pprof "v.io/v23/services/pprof"
	"v.io/v23/verror"
)

// NewPProfService returns a new pprof service implementation.
func NewPProfService() interface{} {
	return s_pprof.PProfServer(&pprofService{})
}

type pprofService struct {
}

// CmdLine returns the command-line argument of the server.
func (pprofService) CmdLine(*context.T, rpc.ServerCall) ([]string, error) {
	return os.Args, nil
}

// Profiles returns the list of available profiles.
func (pprofService) Profiles(*context.T, rpc.ServerCall) ([]string, error) {
	profiles := pprof.Profiles()
	results := make([]string, len(profiles))
	for i, v := range profiles {
		results[i] = v.Name()
	}
	return results, nil
}

// Profile streams the requested profile. The debug parameter enables
// additional output. Passing debug=0 includes only the hexadecimal
// addresses that pprof needs. Passing debug=1 adds comments translating
// addresses to function names and line numbers, so that a programmer
// can read the profile without tools.
func (pprofService) Profile(ctx *context.T, call s_pprof.PProfProfileServerCall, name string, debug int32) error {
	profile := pprof.Lookup(name)
	if profile == nil {
		return fmt.Errorf("profile does not exist: %v", name)
	}
	if err := profile.WriteTo(&streamWriter{call.SendStream()}, int(debug)); err != nil {
		return verror.Convert(verror.ErrUnknown, ctx, err)
	}
	return nil
}

// CPUProfile enables CPU profiling for the requested duration and
// streams the profile data.
//nolint:golint // API change required.
func (pprofService) CpuProfile(ctx *context.T, call s_pprof.PProfCpuProfileServerCall, seconds int32) error {
	if seconds <= 0 || seconds > 3600 {
		return fmt.Errorf("invalid number of seconds: %v, not in range 0...3600", seconds)
	}
	if err := pprof.StartCPUProfile(&streamWriter{call.SendStream()}); err != nil {
		return verror.Convert(verror.ErrUnknown, ctx, err)
	}
	time.Sleep(time.Duration(seconds) * time.Second)
	pprof.StopCPUProfile()
	return nil
}

// Symbol looks up the program counters and returns their respective
// function names.
func (pprofService) Symbol(_ *context.T, _ rpc.ServerCall, programCounters []uint64) ([]string, error) {
	results := make([]string, len(programCounters))
	for i, v := range programCounters {
		f := runtime.FuncForPC(uintptr(v))
		if f != nil {
			results[i] = f.Name()
		}
	}
	return results, nil
}

type streamWriter struct {
	sender interface {
		Send(item []byte) error
	}
}

func (w *streamWriter) Write(p []byte) (int, error) {
	if err := w.sender.Send(p); err != nil {
		return 0, err
	}
	return len(p), nil
}

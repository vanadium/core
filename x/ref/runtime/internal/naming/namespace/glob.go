// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package namespace

import (
	"io"
	"strings"
	"sync"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/glob"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"
)

type tracks struct {
	m      sync.Mutex
	places map[string]struct{}
}

func (tr *tracks) beenThereDoneThat(servers []naming.MountedServer, pstr string) bool {
	tr.m.Lock()
	defer tr.m.Unlock()
	found := false
	for _, s := range servers {
		x := naming.JoinAddressName(s.Server, "") + "!" + pstr
		if _, ok := tr.places[x]; ok {
			found = true
		}
		tr.places[x] = struct{}{}
	}
	return found
}

// task is a sub-glob that has to be performed against a mount table.  Tasks are
// done in parallel to speed up the glob.
type task struct {
	pattern *glob.Glob         // pattern to match
	er      *naming.GlobError  // error for that particular point in the name space
	me      *naming.MountEntry // server to match at
	error   error              // any error performing this task
	depth   int                // number of mount tables traversed recursively
}

// globAtServer performs a Glob on the servers at a mount point.  It cycles through the set of
// servers until it finds one that replies.
func (ns *namespace) globAtServer(ctx *context.T, t *task, replies chan *task, tr *tracks, opts []rpc.CallOpt) {
	defer func() {
		if t.error == nil {
			replies <- nil
		} else {
			replies <- t
		}
	}()
	client := v23.GetClient(ctx)
	pstr := t.pattern.String()
	ctx.VI(2).Infof("globAtServer(%v, %v)", *t.me, pstr)

	// If there are no servers to call, this isn't a mount point.  No sense
	// trying to call servers that aren't there.
	if len(t.me.Servers) == 0 {
		t.error = nil
		return
	}

	// If we've been there before with the same request, give up.
	if tr.beenThereDoneThat(t.me.Servers, pstr) {
		t.error = nil
		return
	}

	// t.me.Name has already been matched at this point to so don't pass it to the Call.  Kind of sleazy to do this
	// but it avoids making yet another copy of the MountEntry.
	on := t.me.Name
	t.me.Name = ""
	timeoutCtx, cancel := withTimeout(ctx)
	defer cancel()
	call, err := client.StartCall(timeoutCtx, "", rpc.GlobMethod, []interface{}{pstr}, append(opts, options.Preresolved{Resolution: t.me})...)
	t.me.Name = on
	if err != nil {
		t.error = err
		return
	}

	// At this point we're committed to the server that answered the call
	// first. Cycle through all replies from that server.
	for {
		// If the mount table returns an error, we're done.  Send the task to the channel
		// including the error.  This terminates the task.
		var gr naming.GlobReply
		err := call.Recv(&gr)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.error = err
			return
		}

		var x *task
		switch v := gr.(type) {
		case naming.GlobReplyEntry:
			// Convert to the ever so slightly different name.MountTable version of a MountEntry
			// and add it to the list.
			x = &task{
				me: &naming.MountEntry{
					Name:             naming.Join(t.me.Name, v.Value.Name),
					Servers:          v.Value.Servers,
					ServesMountTable: v.Value.ServesMountTable,
					IsLeaf:           v.Value.IsLeaf,
				},
				depth: t.depth + 1,
			}
		case naming.GlobReplyError:
			// Pass on the error.
			x = &task{
				er:    &v.Value,
				depth: t.depth + 1,
			}
		}

		// x.depth is the number of servers we've walked through since we've gone
		// recursive (i.e. with pattern length of 0).  Limit the depth of globs.
		// TODO(p): return an error?
		if t.pattern.Len() == 0 {
			if x.depth > ns.maxRecursiveGlobDepth {
				continue
			}
		}
		replies <- x
	}
	t.error = call.Finish()
}

// depth returns the directory depth of a given name.  It is used to pick off the unsatisfied part of the pattern.
func depth(name string) int {
	name = strings.Trim(naming.Clean(name), "/")
	if name == "" {
		return 0
	}
	return strings.Count(name, "/") + 1
}

// globLoop fires off a go routine for each server and reads backs replies.
func (ns *namespace) globLoop(ctx *context.T, e *naming.MountEntry, prefix string, pattern *glob.Glob, reply chan naming.GlobReply, tr *tracks, opts []rpc.CallOpt) {
	defer close(reply)

	// Provide enough buffers to avoid too much switching between the readers and the writers.
	// This size is just a guess.
	replies := make(chan *task, 100)
	defer close(replies)

	// Push the first task into the channel to start the ball rolling.  This task has the
	// root of the search and the full pattern.  It will be the first task fired off in the for
	// loop that follows.
	replies <- &task{me: e, pattern: pattern}
	replies <- nil
	inFlight := 1

	// Perform a parallel search of the name graph.  Each task will send what it learns
	// on the replies channel.  If the reply is a mount point and the pattern is not completely
	// fulfilled, a new task will be fired off to handle it.
	for inFlight != 0 {
		t := <-replies
		// A nil reply represents a successfully terminated task.
		// If no tasks are running, return.
		if t == nil {
			inFlight--
			continue
		}

		// We want to output this entry if there was a real error other than
		// "not a mount table".
		//
		// An error reply is also a terminated task.
		// If no tasks are running, return.
		if t.error != nil {
			if !notAnMT(t.error) {
				reply <- &naming.GlobReplyError{Value: naming.GlobError{Name: naming.Join(prefix, t.me.Name), Error: t.error}}
			}
			inFlight--
			continue
		}

		// If this is just an error from the mount table, pass it on.
		if t.er != nil {
			x := *t.er
			x.Name = naming.Join(prefix, x.Name)
			reply <- &naming.GlobReplyError{Value: x}
			continue
		}

		// Get the pattern elements below the current path.
		suffix := pattern
		for i := depth(t.me.Name) - 1; i >= 0; i-- {
			suffix = suffix.Tail()
		}

		// If we've satisfied the request and this isn't the root,
		// reply to the caller.
		if suffix.Len() == 0 && t.depth != 0 {
			x := *t.me
			x.Name = naming.Join(prefix, x.Name)
			reply <- &naming.GlobReplyEntry{Value: x}
		}

		// If the pattern is finished (so we're only querying about the root on the
		// remote server) and the server is not another MT, then we needn't send the
		// query on since we know the server will not supply a new address for the
		// current name.
		if suffix.Empty() {
			if !t.me.ServesMountTable {
				continue
			}
		}

		// If this is restricted recursive and not a mount table, don't descend into it.
		if suffix.Restricted() && suffix.Len() == 0 && !t.me.ServesMountTable {
			continue
		}

		// Perform a glob at the next server.
		inFlight++
		t.pattern = suffix
		go ns.globAtServer(ctx, t, replies, tr, opts)
	}
}

// Glob implements naming.MountTable.Glob.
func (ns *namespace) Glob(ctx *context.T, pattern string, opts ...naming.NamespaceOpt) (<-chan naming.GlobReply, error) {
	// Root the pattern.  If we have no servers to query, give up.
	e, patternWasRooted := ns.rootMountEntry(pattern)
	if len(e.Servers) == 0 {
		return nil, naming.ErrNoMountTable.Errorf(ctx, "no mounttable")
	}

	// If the name doesn't parse, give up.
	g, err := glob.Parse(e.Name)
	if err != nil {
		return nil, err
	}

	tr := &tracks{places: make(map[string]struct{})}

	// If pattern was already rooted, make sure we tack that root
	// onto all returned names.  Otherwise, just return the relative
	// name.
	var prefix string
	if patternWasRooted {
		prefix = e.Servers[0].Server
	}
	e.Name = ""
	reply := make(chan naming.GlobReply, 100)
	go ns.globLoop(ctx, e, prefix, g, reply, tr, getCallOpts(opts))
	return reply, nil
}

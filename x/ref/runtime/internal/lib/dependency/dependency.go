// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package dependency keeps track of a dependency graph.
// You add edges to the graph by specifying an object and the objects it depends on.
// You can then call FinsihAndWait when the object is finished to wait until
// all the dependents are also finished.
package dependency

import (
	"fmt"
	"sync"
)

var ErrNotFound = fmt.Errorf(
	"attempting to create an object whose dependency has already been terminated")

// Every object in a Graph depends on the all key.  We can wait on this key
// to know when all objects have been closed.
type all struct{}

type node struct {
	dependents int
	cond       *sync.Cond
	dependsOn  []*node
}

// Graph keeps track of a number of objects and their dependents.
// Typical usage looks like:
//
// g := NewGraph()
//
// // Instruct the graph that A depends on B and C.
// if err := g.Depend(A, B, C); err != nil {
//   // Oops, B or C is already terminating, clean up A immediately.
// }
// // D depends on A (You should check the error as above).
// g.Depend(D, A)
// ...
// // At some point we want to mark A as closed to new users and
// // wait for all the objects that depend on it to finish
// // (in this case D).
// finish := g.CloseAndWait(A)
// // Now we know D (and any other depdendents) are finished, so we
// // can clean up A.
// A.CleanUp()
// // Now notify the objects A depended on that they have one less
// // dependent.
// finish()
type Graph struct {
	mu    sync.Mutex
	nodes map[interface{}]*node
}

// NewGraph returns a new Graph ready to be used.
func NewGraph() *Graph {
	graph := &Graph{nodes: map[interface{}]*node{}}
	graph.nodes[all{}] = &node{cond: sync.NewCond(&graph.mu)}
	return graph
}

// Depend adds obj as a node in the dependency graph and notes it's
// dependencies on all the objects in 'on'.  If any of the
// dependencies are already closed (or are not in the graph at all)
// then Depend returns ErrNotFound and does not add any edges.
func (g *Graph) Depend(obj interface{}, on ...interface{}) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	nodes := make([]*node, len(on)+1)
	for i := range on {
		if nodes[i] = g.nodes[on[i]]; nodes[i] == nil {
			return ErrNotFound
		}
	}
	alln := g.nodes[all{}]
	if alln == nil {
		return ErrNotFound
	}
	nodes[len(on)] = alln

	for _, n := range nodes {
		n.dependents++
	}
	if n := g.nodes[obj]; n != nil {
		n.dependsOn = append(n.dependsOn, nodes...)
	} else {
		g.nodes[obj] = &node{
			cond:      sync.NewCond(&g.mu),
			dependsOn: nodes,
		}
	}
	return nil
}

// CloseAndWait closes an object to new dependents and waits for all
// dependants to complete.  When this function returns you can safely
// clean up Obj knowing that no users remain.  Once obj is finished
// with the objects it depends on, you should call the returned function.
func (g *Graph) CloseAndWait(obj interface{}) func() {
	g.mu.Lock()
	defer g.mu.Unlock()
	n := g.nodes[obj]
	if n == nil {
		return func() {}
	}
	delete(g.nodes, obj)
	for n.dependents > 0 {
		n.cond.Wait()
	}
	return func() {
		g.mu.Lock()
		defer g.mu.Unlock()
		for _, dn := range n.dependsOn {
			if dn.dependents--; dn.dependents == 0 {
				dn.cond.Broadcast()
			}
		}
	}
}

// CloseAndWaitForAll closes the graph.  No new objects or dependencies can be added
// and this function returns only after all existing objects have called
// Finish on their finishers.
func (g *Graph) CloseAndWaitForAll() {
	g.CloseAndWait(all{})
}

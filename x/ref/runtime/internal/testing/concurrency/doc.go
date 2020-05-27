// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package concurrency implements a framework for systematic testing
// of concurrent vanadium Go programs. The framework implements the
// ideas described in "Systematic and Scalable Testing of Concurrent
// Programs":
//
// http://repository.cmu.edu/cgi/viewcontent.cgi?article=1291&context=dissertations
//
// Abstractly, the systematic testing framework divides execution of
// concurrent threads into coarse-grained transitions, by interposing
// on events of interest (e.g. thread creation, mutex acquisition, or
// channel communication).
//
// The interposed events suspended and a centralized user-level
// scheduler is used to serialize the concurrent execution by
// advancing allowing only one concurrent transition to execute at any
// given time. In addition to controlling the scheduling, this
// centralized scheduler keeps track of the alternative scheduling
// choices. This information is then used to explore a different
// sequence of transitions next time the test body is executed.
//
// The framework is initialized through the Init(setup, body, cleanup)
// function which specifies the test setup, body, and cleanup
// respectively. To start a systematic exploration, one invokes one of
// the following functions: Explore(), ExploreN(n), or
// ExploreFor(d). These functions repeatedly execute the test
// described through Init(), systematically enumerating the different
// ways in which different executions sequence concurrent transitions
// of the test. Finally, each systematic exploration should end by
// invoking the Finish() function.
//
// See mutex_test.go for an example on how to use this framework to
// test concurrent access to mutexes.
package concurrency

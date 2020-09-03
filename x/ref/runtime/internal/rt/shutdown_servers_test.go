// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rt_test

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/x/lib/gosh"
	"v.io/x/ref/lib/signals"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
)

type dummy struct{}

func (*dummy) Echo(*context.T, rpc.ServerCall) error { return nil }

// remoteCmdLoop listens on stdin and interprets commands sent over stdin (from
// the parent process).
func remoteCmdLoop(ctx *context.T, stdin io.Reader) func() {
	done := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(stdin)
		for scanner.Scan() {
			if scanner.Text() == "close" {
				close(done)
				return
			}
		}
	}()
	return func() { <-done }
}

// complexServerProgram demonstrates the recommended way to write a more
// complex server application (with several servers, a mix of interruptible
// and blocking cleanup, and parallel and sequential cleanup execution).
// For a more typical server, see simpleServerProgram.
var complexServerProgram = gosh.RegisterFunc("complexServerProgram", func() {
	// Initialize the runtime.  This is boilerplate.
	ctx, shutdown := test.V23Init()
	// shutdown is optional, but it's a good idea to clean up, especially
	// since it takes care of flushing the logs before exiting.
	defer shutdown()

	// This is part of the test setup -- we need a way to accept
	// commands from the parent process to simulate Stop and
	// RemoteStop commands that would normally be issued from
	// application code.
	defer remoteCmdLoop(ctx, os.Stdin)()

	// Create a couple servers, and start serving.
	ctx, cancel := context.WithCancel(ctx)
	ctx, server1, err := v23.WithNewServer(ctx, "", &dummy{}, nil)
	if err != nil {
		ctx.Fatalf("r.NewServer error: %s", err)
	}
	ctx, server2, err := v23.WithNewServer(ctx, "", &dummy{}, nil)
	if err != nil {
		ctx.Fatalf("r.NewServer error: %s", err)
	}

	// This is how to wait for a shutdown.  In this example, a shutdown
	// comes from a signal or a stop command.
	var done sync.WaitGroup
	done.Add(1)

	// This is how to configure signal handling to allow clean shutdown.
	sigChan := make(chan os.Signal, 2)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	// Blocking is used to prevent the process from exiting upon receiving a
	// second signal or stop command while critical cleanup code is
	// executing.
	var blocking sync.WaitGroup
	blockingCh := make(chan struct{})

	// This is how to wait for a signal or stop command and initiate the
	// clean shutdown steps.
	go func() {
		// First signal received.
		sig := <-sigChan
		// If the developer wants to take different actions
		// depending on the type of signal, they can do it here.
		fmt.Println("Received signal", sig)
		// This commences the cleanup stage.
		done.Done()
		// Wait for a second signal or stop command, and force an exit,
		// but only once all blocking cleanup code (if any) has
		// completed.
		<-sigChan
		<-blockingCh
		os.Exit(signals.DoubleStopExitCode)
	}()

	// This communicates to the parent test driver process in our unit test
	// that this server is ready and waiting on signals or stop commands.
	// It's purely an artifact of our test setup.
	fmt.Println("Ready")

	// Wait for shutdown.
	done.Wait()

	// Stop the servers.
	cancel()
	<-server1.Closed()
	<-server2.Closed()

	// This is where all cleanup code should go.  By placing it at the end,
	// we make its purpose and order of execution clear.

	// This is an example of how to mix parallel and sequential cleanup
	// steps.  Most real-world servers will likely be simpler, with either
	// just sequential or just parallel cleanup stages.

	// parallelCleanup is used to wait for all goroutines executing cleanup
	// code in parallel to finish.
	var parallelCleanup sync.WaitGroup

	// Simulate four parallel cleanup steps, two blocking and two
	// interruptible.
	parallelCleanup.Add(1)
	blocking.Add(1)
	go func() {
		fmt.Println("Parallel blocking cleanup1")
		blocking.Done()
		parallelCleanup.Done()
	}()

	parallelCleanup.Add(1)
	blocking.Add(1)
	go func() {
		fmt.Println("Parallel blocking cleanup2")
		blocking.Done()
		parallelCleanup.Done()
	}()

	parallelCleanup.Add(1)
	go func() {
		fmt.Println("Parallel interruptible cleanup1")
		parallelCleanup.Done()
	}()

	parallelCleanup.Add(1)
	go func() {
		fmt.Println("Parallel interruptible cleanup2")
		parallelCleanup.Done()
	}()

	// Simulate two sequential cleanup steps, one blocking and one
	// interruptible.
	fmt.Println("Sequential blocking cleanup")
	blocking.Wait()
	close(blockingCh)

	fmt.Println("Sequential interruptible cleanup")

	parallelCleanup.Wait()
})

// simpleServerProgram demonstrates the recommended way to write a typical
// simple server application (with one server and a clean shutdown triggered by
// a signal or a stop command).  For an example of something more involved, see
// complexServerProgram.
var simpleServerProgram = gosh.RegisterFunc("simpleServerProgram", func() {
	// Initialize the runtime.  This is boilerplate.
	ctx, shutdown := test.V23Init()
	// Calling shutdown is optional, but it's a good idea to clean up, especially
	// for servers to remove their entries from the mounttable and to ensure
	// flushing the logs before exiting.
	//
	// We use defer to ensure this is the last thing in the program (to
	// avoid shutting down the runtime while it may still be in use), and to
	// allow it to execute even if a panic occurs down the road.
	defer shutdown()

	// This is how to wait for a shutdown.  In this example, a shutdown
	// comes from a signal or context cancelation command.
	//
	// Note, if the developer wants to exit immediately upon receiving a
	// signal or stop command, they can skip this, in which case the default
	// behavior is for the process to exit.
	ctx, handler := signals.ShutdownOnSignalsWithCancel(ctx)

	// This is part of the test setup -- we need a way to accept
	// commands from the parent process to simulate Stop and
	// RemoteStop commands that would normally be issued from
	// application code.
	defer remoteCmdLoop(ctx, os.Stdin)()

	// Create a server, and start serving.
	ctx, server, err := v23.WithNewServer(ctx, "", &dummy{}, nil)
	if err != nil {
		ctx.Fatalf("r.NewServer error: %s", err)
	}

	// This communicates to the parent test driver process in our unit test
	// that this server is ready and waiting on signals or stop commands.
	// It's purely an artifact of our test setup.
	fmt.Println("Ready")

	// Use defer for anything that should still execute even if a panic
	// occurs.
	defer fmt.Println("Deferred cleanup")

	// Wait for shutdown, which will also cancel the context
	// when a signal is received.
	sig := handler.WaitForSignal()

	// Note: this is not strictly required since the defer'ed shutdown
	// function will also call <- server.Closed()
	<-server.Closed()

	// The developer could take different actions depending on the type of
	// signal.
	fmt.Println("Received signal", sig)

	// Note, this will not execute in cases of forced shutdown
	// (e.g. SIGSTOP), when the process calls os.Exit (e.g. via log.Fatal),
	// or when a panic occurs.
	fmt.Println("Interruptible cleanup")
})

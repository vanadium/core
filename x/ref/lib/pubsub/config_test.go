// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pubsub_test

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"v.io/x/ref/lib/pubsub"
)

func ExamplePublisher() {
	in := make(chan pubsub.Setting)
	pub := pubsub.NewPublisher()
	pub.CreateStream("net", "network settings", in) //nolint:errcheck

	// A simple producer of IP address settings.
	producer := func() {
		in <- pubsub.NewString("ip", "address", "1.2.3.5")
	}

	var waiter sync.WaitGroup
	waiter.Add(2)

	// A simple consumer of IP address Settings.
	consumer := func(ch chan pubsub.Setting) {
		fmt.Println(<-ch)
		waiter.Done()
	}

	// Publish an initial Setting to the Stream.
	in <- pubsub.NewString("ip", "address", "1.2.3.4")

	// Fork the stream twice, and read the latest value.
	ch1 := make(chan pubsub.Setting)
	st, _ := pub.ForkStream("net", ch1)
	fmt.Println(st.Latest["ip"])
	ch2 := make(chan pubsub.Setting)
	st, _ = pub.ForkStream("net", ch2)
	fmt.Println(st.Latest["ip"])

	// Now we can read new Settings as they are generated.
	go producer()
	go consumer(ch1)
	go consumer(ch2)

	waiter.Wait()

	// Output:
	// ip: address: (string: 1.2.3.4)
	// ip: address: (string: 1.2.3.4)
	// ip: address: (string: 1.2.3.5)
	// ip: address: (string: 1.2.3.5)
}

func ExamplePublisher_Shutdown() {
	in := make(chan pubsub.Setting)
	pub := pubsub.NewPublisher()
	stop, _ := pub.CreateStream("net", "network settings", in)

	var producerReady sync.WaitGroup
	producerReady.Add(1)
	var consumersReady sync.WaitGroup
	consumersReady.Add(2)

	// A producer to write 100 Settings before signalling that it's
	// ready to be shutdown. This is purely to demonstrate how to use
	// Shutdown.
	producer := func() {
		for i := 0; ; i++ {
			select {
			case <-stop:
				close(in)
				return
			default:
				in <- pubsub.NewString("ip", "address", "1.2.3.4")
				if i == 100 {
					producerReady.Done()
				}
			}
		}
	}

	var waiter sync.WaitGroup
	waiter.Add(2)

	consumer := func() {
		ch := make(chan pubsub.Setting, 10)
		pub.ForkStream("net", ch) //nolint:errcheck
		consumersReady.Done()
		i := 0
		for {
			if _, ok := <-ch; !ok {
				// The channel has been closed when the publisher
				// is asked to shut down.
				break
			}
			i++
		}
		if i >= 100 {
			// We've received at least 100 Settings as per the producer above.
			fmt.Println("done")
		}
		waiter.Done()
	}

	go consumer()
	go consumer()
	consumersReady.Wait()
	go producer()
	producerReady.Wait()
	pub.Shutdown()
	waiter.Wait()
	// Output:
	// done
	// done
}

func TestSimple(t *testing.T) { //nolint:gocyclo
	ch := make(chan pubsub.Setting, 2)
	pub := pubsub.NewPublisher()
	if _, err := pub.ForkStream("stream", nil); err == nil || !strings.Contains(err.Error(), "stream stream doesn't exist") {
		t.Errorf("missing or wrong error: %v", err)
	}
	if _, err := pub.CreateStream("stream", "example", nil); err == nil || !strings.Contains(err.Error(), "must provide a non-nil channel") {
		t.Fatalf("missing or wrong error: %v", err)
	}
	if _, err := pub.CreateStream("stream", "example", ch); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if _, err := pub.CreateStream("stream", "example", ch); err == nil || !strings.Contains(err.Error(), "stream stream already exists") {
		t.Fatalf("missing or wrong error: %v", err)
	}
	if got, want := pub.String(), "(stream: example)"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	stop, err := pub.CreateStream("s2", "eg2", ch)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if got, want := pub.String(), "(stream: example) (s2: eg2)"; got != want {
		wantAlternate := "(s2: eg2) (stream: example)"
		if got != wantAlternate {
			t.Errorf("got %q, want %q or %q", got, want, wantAlternate)
		}
	}

	got, want := pub.Latest("s2"), &pubsub.Stream{
		Name:        "s2",
		Description: "eg2",
		Latest:      nil,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	pub.Shutdown()
	if _, running := <-stop; running {
		t.Errorf("expected to be shutting down")
	}
	if _, err := pub.ForkStream("stream", nil); err == nil || !strings.Contains(err.Error(), "stream stream has been shut down") {
		t.Errorf("missing or wrong error: %v", err)
	}
	if got, want := pub.String(), "shutdown"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}

}

func producer(pub *pubsub.Publisher, in chan<- pubsub.Setting, stop <-chan struct{}, limit int, ch chan int, waiter *sync.WaitGroup) {
	for i := 0; ; i++ {
		select {
		case <-stop:
			ch <- i
			waiter.Done()
			// must close this channel, otherwise the Publisher will leak a goroutine for this stream.
			close(in)
			return
		default:
			// signal progress on ch, at limit/2, limit and when we're done (above)
			switch {
			case i == limit/2:
				ch <- i
			case i == limit:
				ch <- i
			}
			if i%2 == 0 {
				in <- pubsub.NewInt("i", "int", i)
			} else {
				in <- pubsub.NewFloat64("f", "float", float64(i))
			}
		}
	}
}

func consumer(t *testing.T, pub *pubsub.Publisher, limit, bufsize int, errch chan error, starter, waiter *sync.WaitGroup) {
	defer close(errch)
	ch := make(chan pubsub.Setting, bufsize)
	st, err := pub.ForkStream("net", ch)
	if err != nil {
		errch <- err
		return
	}
	starter.Done()
	i, i2 := 0, 0
	if st.Latest["i"] != nil {
		i = st.Latest["i"].Value().(int)
	}
	if st.Latest["f"] != nil {
		i2 = int(st.Latest["f"].Value().(float64))
	}
	if i2 > i {
		i = i2
	}
	i++
	for s := range ch {
		switch v := s.Value().(type) {
		case int:
			if i%2 != 0 {
				errch <- fmt.Errorf("expected a float, got an int")
				return
			}
			if v != i {
				errch <- fmt.Errorf("got %d, want %d", v, i)
				return
			}
		case float64:
			if i%2 != 1 {
				errch <- fmt.Errorf("expected an int, got a float")
				return
			}
			if v != float64(i) {
				errch <- fmt.Errorf("got %f, want %f", v, float64(i))
				return
			}
		}
		i++
	}
	if i < limit {
		errch <- fmt.Errorf("didn't read enough settings: got %d, want >= %d", i, limit)
		return
	}
	waiter.Done()
}

func testStream(t *testing.T, consumerBufSize int) {
	in := make(chan pubsub.Setting)
	pub := pubsub.NewPublisher()
	stop, err := pub.CreateStream("net", "network settings", in)
	if err != nil {
		t.Fatal(err)
	}

	rand.Seed(time.Now().UnixNano())
	limit := rand.Intn(5000)
	if limit < 100 {
		limit = 100
	}
	t.Logf("limit: %d", limit)

	var waiter sync.WaitGroup
	waiter.Add(3)

	progress := make(chan int)
	go producer(pub, in, stop, limit, progress, &waiter)

	i := <-progress
	t.Logf("limit/2 = %d", i)

	err1 := make(chan error, 1)
	err2 := make(chan error, 1)
	var starter sync.WaitGroup
	starter.Add(2)
	go consumer(t, pub, limit, consumerBufSize, err1, &starter, &waiter)
	go consumer(t, pub, limit, consumerBufSize, err2, &starter, &waiter)

	reached := <-progress
	// Give the consumers a chance to get going before shutting down
	// the producer.
	starter.Wait()
	time.Sleep(100 * time.Millisecond)
	pub.Shutdown()
	shutdown := <-progress
	t.Logf("reached %d, shut down at %d", reached, shutdown)

	// This is a little annoying, we check for the presence of errors on the error
	// channels once everything has run its course  since it's not allowed to call
	// t.Fatal/Errorf from a separate goroutine. We wait until here so that we
	// don't block waiting for errors that don't occur when the tests all work.
	err = <-err1
	if err != nil {
		t.Fatal(err)
	}
	err = <-err2
	if err != nil {
		t.Fatal(err)
	}
	// Wait for all goroutines to finish.
	waiter.Wait()
}

func TestStream(t *testing.T) {
	testStream(t, 500)
}

func TestStreamSmallBuffers(t *testing.T) {
	testStream(t, 1)
}

func TestDurationFlag(t *testing.T) {
	d := &pubsub.DurationFlag{}
	if err := d.Set("1s"); err != nil {
		t.Errorf("unexpected error %s", err)
	}
	if got, want := d.Duration, time.Second; got != want {
		t.Errorf("got %s, expected %s", got, want)
	}
	if err := d.Set("1t"); err == nil || !strings.Contains(err.Error(), "time: unknown unit") {
		t.Errorf("expected error %v", err)
	}
}

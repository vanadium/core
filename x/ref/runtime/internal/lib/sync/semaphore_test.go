// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestSemaphore(t *testing.T) {
	s1 := NewSemaphore()
	s2 := NewSemaphore()

	var pending sync.WaitGroup
	pending.Add(1)
	go func() {
		//nolint:errcheck
		s1.Dec(nil)
		s2.Inc()
		pending.Done()
	}()

	s1.Inc()
	if err := s2.Dec(nil); err != nil {
		t.Fatal(err)
	}
	pending.Wait()
}

func TestCriticalSection(t *testing.T) {
	s := NewSemaphore()
	s.Inc()

	var count int32
	var pending sync.WaitGroup
	pending.Add(100)
	for i := 0; i != 100; i++ {
		go func() {
			for j := 0; j != 100; j++ {
				if err := s.Dec(nil); err != nil {
					t.Errorf("dec: %v", err)
				}
				// Critical section.
				v := atomic.AddInt32(&count, 1)
				if v > 1 {
					t.Errorf("Race detected")
				}
				atomic.AddInt32(&count, -1)
				s.Inc()
			}
			pending.Done()
		}()
	}
	pending.Wait()
}

func TestIncN(t *testing.T) {
	s := NewSemaphore()
	// done is used to synchronize the decrementer with the incrementer.  Each
	// time Dec is called, a value is sent on Done.  The incrementer waits for
	// these messages to check that the decrement has happened.
	done := make(chan struct{})
	for i := 0; i != 100; i++ {
		go func() {
			if err := s.Dec(nil); err != nil {
				t.Errorf("dec: %v", err)
			}
			done <- struct{}{}
		}()
	}

	// Produce in blocks of 10.
	for i := 0; i != 100; i += 10 {
		s.IncN(10)
		for j := 0; j != 10; j++ {
			<-done
		}
	}
}

func TestTryDecN(t *testing.T) {
	s := NewSemaphore()
	err := s.TryDecN(0)
	if err != nil {
		t.Errorf("TryDecN: %s", err)
	}
	err = s.TryDecN(1)
	if err == nil {
		t.Errorf("TryDecN: expected error")
	}
	s.IncN(1)
	err = s.TryDecN(2)
	if err == nil {
		t.Errorf("TryDecN: expected error")
	}
	err = s.TryDecN(1)
	if err != nil {
		t.Errorf("TryDecN: %s", err)
	}

	s.IncN(5)
	err = s.TryDecN(3)
	if err != nil {
		t.Errorf("TryDecN: %s", err)
	}
	err = s.TryDecN(3)
	if err == nil {
		t.Errorf("TryDecN: expected error")
	}
}

func TestClosed(t *testing.T) {
	s := NewSemaphore()
	var pending sync.WaitGroup
	pending.Add(1)
	go func() {
		if err := s.Dec(nil); err == nil {
			t.Errorf("error should not be nil")
		}
		pending.Done()
	}()
	s.Close()
	pending.Wait()
}

func TestCloseWhileInDec(t *testing.T) {
	const N = 100
	s := NewSemaphore()
	var pending sync.WaitGroup
	pending.Add(N)

	dec := func(idx int) {
		defer pending.Done()
		if err := s.Dec(nil); err != nil {
			t.Fatalf("%v (%d)", err, idx)
		}
	}

	for i := 0; i < N; i++ {
		go dec(i)
	}
	s.IncN(N + 1)
	s.Close()
	pending.Wait()
	if got, want := s.DecN(2, nil), ErrClosed; got != want {
		t.Errorf("Expected error %v, got %v instead", want, got)
	}
}

func TestCancel(t *testing.T) {
	s := NewSemaphore()
	cancel := make(chan struct{})
	var pending sync.WaitGroup
	pending.Add(1)
	go func() {
		if err := s.Dec(cancel); err == nil {
			t.Errorf("error should not be nil")
		}
		pending.Done()
	}()
	close(cancel)
	pending.Wait()
}

func BenchmarkInc(b *testing.B) {
	s := NewSemaphore()
	for i := 0; i < b.N; i++ {
		s.Inc()
	}
}

func BenchmarkDec(b *testing.B) {
	s := NewSemaphore()
	cancel := make(chan struct{})
	s.IncN(uint(b.N))
	for i := 0; i < b.N; i++ {
		if err := s.Dec(cancel); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecCanceled(b *testing.B) {
	s := NewSemaphore()
	cancel := make(chan struct{})
	close(cancel)
	for i := 0; i < b.N; i++ {
		if err := s.Dec(cancel); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTryDec_TryAgain(b *testing.B) {
	s := NewSemaphore()
	for i := 0; i < b.N; i++ {
		if err := s.TryDecN(1); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTryDec_Success(b *testing.B) {
	s := NewSemaphore()
	s.IncN(uint(b.N))
	for i := 0; i < b.N; i++ {
		if err := s.TryDecN(1); err != nil {
			b.Fatalf("TryDecN failed with %v in iteration %d", err, i)
			return
		}
	}
}

func BenchmarkIncDecInterleaved(b *testing.B) {
	c := make(chan bool)
	s := NewSemaphore()
	go func() {
		for i := 0; i < b.N; i++ {
			s.Dec(nil) //nolint:errcheck
		}
		c <- true
	}()
	go func() {
		for i := 0; i < b.N; i++ {
			s.Inc()
		}
		c <- true
	}()
	<-c
	<-c
}

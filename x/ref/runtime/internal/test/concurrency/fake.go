// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package concurrency

// mutexState is a type to represent different states of a mutex.
type mutexState uint64

// enumeration of different mutex states.
const (
	mutexFree mutexState = iota
	mutexLocked
)

// fakeMutex is an abstract representation of a mutex.
type fakeMutex struct {
	// clock records the logical time of the last access to the mutex.
	clock clock
	// state records the state of the mutex.
	state mutexState
}

// newFakeMutex is the fakeMutex factory.
func newFakeMutex(clock clock) *fakeMutex {
	return &fakeMutex{clock: clock.clone()}
}

// free checks if the mutex is free.
func (m *fakeMutex) free() bool {
	return m.state == mutexFree
}

// locked checks if the mutex is locked.
func (m *fakeMutex) locked() bool {
	return m.state == mutexLocked
}

// lock models the action of locking the mutex.
func (m *fakeMutex) lock() {
	if m.state != mutexFree {
		panic("Locking a mutex that is already locked.")
	}
	m.state = mutexLocked
}

// unlock models the action of unlocking the mutex.
func (m *fakeMutex) unlock() {
	if m.state != mutexLocked {
		panic("Unlocking a mutex that is not locked.")
	}
	m.state = mutexFree
}

// rwMutexState is a type to represent different states of a mutex.
type rwMutexState uint64

// enumeration of different rwMutex states.
const (
	rwMutexFree rwMutexState = iota
	rwMutexShared
	rwMutexExclusive
)

// fakeRWMutex is an abstract representation of a read-write mutex.
type fakeRWMutex struct {
	// clock records the logical time of the last access to the
	// read-write mutex.
	clock clock
	// state records the state of the read-write mutex.
	state rwMutexState
	// nreaders records the number of readers.
	nreaders int
}

// newFakeRWMutex is the fakeRWMutex factory.
func newFakeRWMutex(clock clock) *fakeRWMutex {
	return &fakeRWMutex{clock: clock.clone()}
}

// exclusive checks if the read-write mutex is exclusive.
func (rw *fakeRWMutex) exclusive() bool {
	return rw.state == rwMutexExclusive
}

// free checks if the read-write mutex is free.
func (rw *fakeRWMutex) free() bool {
	return rw.state == rwMutexFree
}

// shared checks if the read-write mutex is shared.
func (rw *fakeRWMutex) shared() bool {
	return rw.state == rwMutexShared
}

// lock models the action of read-locking or write-locking the
// read-write mutex.
func (rw *fakeRWMutex) lock(read bool) {
	if read {
		if rw.state == rwMutexExclusive {
			panic("Read-locking a read-write mutex that is write-locked.")
		}
		if rw.state == rwMutexFree {
			rw.state = rwMutexShared
		}
		rw.nreaders++
	} else {
		if rw.state != rwMutexFree {
			panic("Write-locking a read-write mutex that is not free.")
		}
		rw.state = rwMutexExclusive
	}
}

// unlock models the action of unlocking the read-write mutex.
func (rw *fakeRWMutex) unlock(read bool) {
	if read {
		if rw.state != rwMutexShared {
			panic("Read-unlocking a read-write mutex that is not read-locked.")
		}
		rw.nreaders--
		if rw.nreaders == 0 {
			rw.state = rwMutexFree
		}
	} else {
		if rw.state != rwMutexExclusive {
			panic("Write-unlocking a read-write mutex that is not write-locked.")
		}
		rw.state = rwMutexFree
	}
}

// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lock

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"v.io/x/lib/gosh"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/agent/internal/lockutil"
	"v.io/x/ref/test/timekeeper"
)

var goshLevelTryLock = gosh.RegisterFunc("goshLevelTryLock", func(dir string, index int) {
	l := newDirLock(dir, timekeeper.NewManualTime())
	switch outcome := l.tryLock(index).(type) {
	default:
		vlog.Fatalf("Unexpected type %T", outcome)
	case grabbedLock:
		if bool(outcome) {
			fmt.Println("grabbed")
		} else {
			fmt.Println("not grabbed")
		}
	case staleLock:
		fmt.Println("stale")
	case failedLock:
		vlog.Fatalf("Failed: %v", outcome.error)
	}
})

var goshTryLock = gosh.RegisterFunc("goshTryLock", func(dir string) {
	l := newDirLock(dir, timekeeper.NewManualTime())
	grabbed, err := l.TryLock()
	if err != nil {
		vlog.Fatalf("Failed: %v", err)
	}
	fmt.Println(map[bool]string{true: "grabbed", false: "not grabbed"}[grabbed])
})

func setup(t *testing.T) (string, *dirLock) {
	d, err := ioutil.TempDir("", "locktest")
	if err != nil {
		t.Fatalf("Failed to create test dir: %v", err)
	}
	return d, newDirLock(d, timekeeper.NewManualTime())
}

// TestSleep verifies the (internal) sleep method.
func TestSleep(t *testing.T) {
	d, l := setup(t)
	defer os.RemoveAll(d)

	tk := l.timeKeeper.(timekeeper.ManualTime)
	for i := 0; i < 100; i++ {
		sleepDone := make(chan struct{})
		go func() {
			l.sleep()
			close(sleepDone)
		}()
		sleepDuration := <-tk.Requests()
		if sleepDuration < sleepTime-sleepJitter || sleepDuration > sleepTime+sleepJitter {
			t.Fatalf("sleep expected to be %v (+/- %v); got %v instead", sleepTime, sleepJitter, sleepDuration)
		}
		tk.AdvanceTime(sleepDuration)
		<-sleepDone
	}
}

// TestLockFile verifies that the (internal) lockFile method returns a lock file
// path in the right directory.
func TestLockFile(t *testing.T) {
	d, l := setup(t)
	defer os.RemoveAll(d)

	f0 := l.lockFile(0)
	if got := filepath.Dir(f0); got != d {
		t.Fatalf("Expected file %s to be in dir %s", f0, d)
	}
	f1 := l.lockFile(1)
	if got := filepath.Dir(f1); got != d {
		t.Fatalf("Expected file %s to be in dir %s", f1, d)
	}
	if f0 == f1 {
		t.Fatalf("Expected lock files for levels 0 and 1 to differ")
	}
}

func verifyFreeLock(t *testing.T, l *dirLock, index int) {
	lockInfo, err := l.readLockInfo(index)
	if !os.IsNotExist(err) {
		t.Fatalf("readLockInfo should have not found the lockfile, got (%v, %v) instead", string(lockInfo), err)
	}
}

func verifyHeldLock(t *testing.T, l *dirLock, index int) {
	lockInfo, err := l.readLockInfo(index)
	if err != nil {
		t.Fatalf("readLockInfo failed: %v", err)
	}
	if running, err := lockutil.StillHeld(lockInfo); err != nil || !running {
		t.Fatalf("Expected (true, <nil>) got (%t, %v) instead from StillHeld for:\n%v", running, err, string(lockInfo))
	}
}

func verifyStaleLock(t *testing.T, l *dirLock, index int) {
	lockInfo, err := l.readLockInfo(index)
	if err != nil {
		t.Fatalf("readLockInfo failed: %v", err)
	}
	if running, err := lockutil.StillHeld(lockInfo); err != nil || running {
		t.Fatalf("Expected (false, <nil>) got (%t, %v) instead from StillHeld for:\n%v", running, err, string(lockInfo))
	}
}

// TestLevelLock tests the (internal) tryLock, unlock, and poachLock methods.
func TestLevelLock(t *testing.T) {
	d, l := setup(t)
	defer os.RemoveAll(d)

	verifyFreeLock(t, l, 0)
	if outcome := l.tryLock(0).(grabbedLock); !bool(outcome) {
		t.Fatalf("Expected lock was grabbed")
	}
	verifyHeldLock(t, l, 0)
	if outcome := l.tryLock(0).(grabbedLock); bool(outcome) {
		t.Fatalf("Expected lock was not grabbed")
	}
	verifyHeldLock(t, l, 0)
	if err := l.unlock(0); err != nil {
		t.Fatalf("unlock failed: %v", err)
	}
	verifyFreeLock(t, l, 0)
	if outcome := l.tryLock(0).(grabbedLock); !bool(outcome) {
		t.Fatalf("Expected lock was grabbed")
	}
	verifyHeldLock(t, l, 0)
	if err := l.poachLock(0); err != nil {
		t.Fatalf("poachLock failed: %v", err)
	}
	verifyHeldLock(t, l, 0)
	if outcome := l.tryLock(1).(grabbedLock); !bool(outcome) {
		t.Fatalf("Expected lock was grabbed")
	}
	verifyHeldLock(t, l, 0)
	verifyHeldLock(t, l, 1)
}

// TestLevelLockStale tests the (internal) tryLock, poachLock, and unlock
// methods in the face of stale locks created by external processes.
func TestLevelLockStale(t *testing.T) {
	d, l := setup(t)
	defer os.RemoveAll(d)
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	verifyFreeLock(t, l, 0)
	if out := sh.FuncCmd(goshLevelTryLock, d, 0).Stdout(); out != "grabbed\n" {
		t.Fatalf("Unexpected output: %s", out)
	}
	verifyStaleLock(t, l, 0)
	if _, ok := l.tryLock(0).(staleLock); !ok {
		t.Fatalf("Expected lock to be stale")
	}
	if out := sh.FuncCmd(goshLevelTryLock, d, 0).Stdout(); out != "stale\n" {
		t.Fatalf("Unexpected output: %s", out)
	}
	if err := l.poachLock(0); err != nil {
		t.Fatalf("poachLock failed: %v", err)
	}
	verifyHeldLock(t, l, 0)
	if out := sh.FuncCmd(goshLevelTryLock, d, 0).Stdout(); out != "not grabbed\n" {
		t.Fatalf("Unexpected output: %s", out)
	}
	if err := l.unlock(0); err != nil {
		t.Fatalf("unlock failed: %v", err)
	}
	verifyFreeLock(t, l, 0)
	if out := sh.FuncCmd(goshLevelTryLock, d, 0).Stdout(); out != "grabbed\n" {
		t.Fatalf("Unexpected output: %s", out)
	}
}

// TestLock tests the exported TryLocker and TryLockerSafe methods.  This is a
// black-box test (if we ignore the timeKeeper manipulation).
func TestLock(t *testing.T) {
	d, l := setup(t)
	defer os.RemoveAll(d)

	if grabbed, err := l.TryLock(); err != nil || !grabbed {
		t.Fatalf("Expected lock was grabbed, got (%t, %v) instead", grabbed, err)
	}
	if grabbed, err := l.TryLock(); err != nil || grabbed {
		t.Fatalf("Expected lock was not grabbed, got (%t, %v) instead", grabbed, err)
	}
	if err := l.Unlock(); err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}
	if grabbed := l.Must().TryLock(); !grabbed {
		t.Fatalf("Expected lock was grabbed, got %t instead", grabbed)
	}
	if grabbed := l.Must().TryLock(); grabbed {
		t.Fatalf("Expected lock was not grabbed, got %t instead", grabbed)
	}
	l.Must().Unlock()
	if grabbed, err := l.TryLock(); err != nil || !grabbed {
		t.Fatalf("Expected lock was grabbed, got (%t, %v) instead", grabbed, err)
	}
	// Lock is currently held.  Attempt to grab the lock in a goroutine.
	// Should only succeed once we release the lock.
	lockDone := make(chan error, 1)
	go func() {
		lockDone <- l.Lock()
	}()
	// Lock should be blocked, and we should see it going to Sleep
	// regularly, waiting for the lock to be released.
	tk := l.timeKeeper.(timekeeper.ManualTime)
	for i := 0; i < 10; i++ {
		select {
		case <-lockDone:
			t.Fatalf("Didn't expect lock to have been grabbed (iteration %d)", i)
		default:

			tk.AdvanceTime(<-tk.Requests())
		}
	}
	if err := l.Unlock(); err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}
	// The lock should now be available for the Lock call to complete.
	if err := <-lockDone; err != nil {
		t.Fatalf("Lock failed: %v", err)
	}
	l.Must().Unlock()
	l.Must().Lock()
	if grabbed := l.Must().TryLock(); grabbed {
		t.Fatalf("Expected lock was not grabbed, got %t instead", grabbed)
	}
}

// TestLockStale tests the exported TryLockerSafe methods in the face of stale
// locks created by external processes.  This is a black-box test.
func TestLockStale(t *testing.T) {
	d, l := setup(t)
	defer os.RemoveAll(d)
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	// Lock gets grabbed by the subprocess (which then exits, rendering the
	// lock stale).
	if out := sh.FuncCmd(goshTryLock, d).Stdout(); out != "grabbed\n" {
		t.Fatalf("Unexpected output: %s", out)
	}
	// Stale lock gets replaced by the current process.
	if grabbed, err := l.TryLock(); !grabbed || err != nil {
		t.Fatalf("Expected (true, <nil>) got (%t, %v) instead", grabbed, err)
	}
	// Subprocess finds that the lock is legitimately owned by a live
	// process.
	if out := sh.FuncCmd(goshTryLock, d).Stdout(); out != "not grabbed\n" {
		t.Fatalf("Unexpected output: %s", out)
	}
	if err := l.Unlock(); err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}
	// The lock is available, so the subprocess grabs it.
	if out := sh.FuncCmd(goshTryLock, d).Stdout(); out != "grabbed\n" {
		t.Fatalf("Unexpected output: %s", out)
	}
	// The lock is stale, so the subprocess grabs it.
	if out := sh.FuncCmd(goshTryLock, d).Stdout(); out != "grabbed\n" {
		t.Fatalf("Unexpected output: %s", out)
	}
}

func assertNumLockFiles(t *testing.T, d string, num int) {
	files, err := ioutil.ReadDir(d)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if nfiles := len(files); nfiles != num {
		t.Fatalf("Expected %d file, found %d", num, nfiles)
	}
}

// TestLockLevelStateChanges verifies the behavior of TryLock when faced with a
// variety of level lock states that may change while TryLock is executing.
func TestLockLevelStateChanges(t *testing.T) {
	d, l := setup(t)
	defer os.RemoveAll(d)
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	// Test case 1: Start with clean slate.  The lock is grabbed by the
	// subprocess.
	if out := sh.FuncCmd(goshTryLock, d).Stdout(); out != "grabbed\n" {
		t.Fatalf("Unexpected output: %s", out)
	}
	assertNumLockFiles(t, d, 1)
	// Remember the lock info, to be re-used to simulate stale locks further
	// down.
	staleInfo, err := l.readLockInfo(0)
	if err != nil {
		t.Fatalf("readLockInfo failed: %v", err)
	}
	if err := l.Unlock(); err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}
	assertNumLockFiles(t, d, 0)

	// Test case 2: Start with level 0 lock stale.  The lock is grabbed by the
	// current process.
	if err := ioutil.WriteFile(l.lockFile(0), staleInfo, os.ModePerm); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	assertNumLockFiles(t, d, 1)
	if grabbed, err := l.TryLock(); err != nil || !grabbed {
		t.Fatalf("Expected lock was grabbed, got (%t, %v) instead", grabbed, err)
	}
	assertNumLockFiles(t, d, 1)
	// Remember the lock info, to be re-used to simulate actively held locks
	// further down.
	activeInfo, err := l.readLockInfo(0)
	if err != nil {
		t.Fatalf("readLockInfo failed: %v", err)
	}

	// goTryLock runs TryLock in a goroutine and returns a function that can
	// be used to verify the outcome of TryLock.  Used in subsequent test
	// cases.
	goTryLock := func() func(bool) {
		doneTryLock := make(chan struct {
			grabbed bool
			err     error
		}, 1)
		go func() {
			grabbed, err := l.TryLock()
			doneTryLock <- struct {
				grabbed bool
				err     error
			}{grabbed, err}
		}()
		return func(expected bool) {
			if outcome := <-doneTryLock; outcome.err != nil || outcome.grabbed != expected {
				t.Fatalf("TryLock: expected (%t, <nil>), got (%t, %v) instead", expected, outcome.grabbed, outcome.err)
			}
		}
	}

	// Test case 3: Start with stale level 0 lock, actively held level 1
	// lock.
	if err := ioutil.WriteFile(l.lockFile(0), staleInfo, os.ModePerm); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := ioutil.WriteFile(l.lockFile(1), activeInfo, os.ModePerm); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	assertNumLockFiles(t, d, 2)
	verifyTryLock := goTryLock()
	tk := l.timeKeeper.(timekeeper.ManualTime)
	// TryLock finds lock 1 held, so it sleeps and retries.
	sleepDuration := <-tk.Requests()
	// Unlock level 1, letting TryLock grab it.
	l.unlock(1)
	tk.AdvanceTime(sleepDuration)
	// TryLock managed to grab level 1 and it then reclaims level 0.
	verifyTryLock(true)
	assertNumLockFiles(t, d, 1)

	// Test case 4: Start with stale lock 0, active lock 1 (which then
	// becomes stale).
	if err := ioutil.WriteFile(l.lockFile(0), staleInfo, os.ModePerm); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := ioutil.WriteFile(l.lockFile(1), activeInfo, os.ModePerm); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	assertNumLockFiles(t, d, 2)
	verifyTryLock = goTryLock()
	// TryLock finds lock 1 held, so it sleeps and retries.
	sleepDuration = <-tk.Requests()
	// Change lock 1 to stale.  TryLock should then move on to level 2,
	// which it can grab and reclaim level 0.
	if err := ioutil.WriteFile(l.lockFile(1), staleInfo, os.ModePerm); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	tk.AdvanceTime(sleepDuration)
	verifyTryLock(true)
	assertNumLockFiles(t, d, 1)

	// Test case 5: Start with stale lock 0, stale lock 1, active lock 2
	// (which then becomes unlocked).
	if err := ioutil.WriteFile(l.lockFile(0), staleInfo, os.ModePerm); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := ioutil.WriteFile(l.lockFile(1), staleInfo, os.ModePerm); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := ioutil.WriteFile(l.lockFile(2), activeInfo, os.ModePerm); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	assertNumLockFiles(t, d, 3)
	verifyTryLock = goTryLock()
	// TryLock finds lock 2 held, so it sleeps and retries.
	sleepDuration = <-tk.Requests()
	// Remove lock 2.  TryLock should then grab it, and then reclaim level
	// 0.
	l.unlock(2)
	tk.AdvanceTime(sleepDuration)
	verifyTryLock(true)
	assertNumLockFiles(t, d, 1)

	// Test case 6: Start with stale lock 0, stale lock 1, active lock 2
	// (which then becomes unlocked; but lock 0 also changes to active).
	if err := ioutil.WriteFile(l.lockFile(0), staleInfo, os.ModePerm); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := ioutil.WriteFile(l.lockFile(1), staleInfo, os.ModePerm); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := ioutil.WriteFile(l.lockFile(2), activeInfo, os.ModePerm); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	assertNumLockFiles(t, d, 3)
	verifyTryLock = goTryLock()
	// TryLock finds lock 2 held, so it sleeps and retries.
	sleepDuration = <-tk.Requests()
	l.unlock(2)
	// We use the hook to control the execution of TryLock and allow
	// ourselves to manipulate the lock files 'asynchronously'.
	tryLockCalled := make(chan struct{})
	l.tryLockHook = func() { tryLockCalled <- struct{}{} }
	tk.AdvanceTime(sleepDuration)
	<-tryLockCalled // Back to level 0, TryLock finds it stale.
	// Mark lock 0 actively held.
	if err := ioutil.WriteFile(l.lockFile(0), activeInfo, os.ModePerm); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	<-tryLockCalled // For lock 1.
	<-tryLockCalled // For lock 2.
	// TryLock grabs lock level 2, but finds that lock level 0 has changed,
	// so it sleeps and retries.
	tk.AdvanceTime(<-tk.Requests())
	<-tryLockCalled // Back to level 0.
	// This time, the level 0 lock is found to be actively held.
	verifyTryLock(false)
	assertNumLockFiles(t, d, 2)
	select {
	case <-tk.Requests():
		t.Fatalf("Not expecting any more sleeps")
	default:
	}

	// Test case 7: Start with stale lock 0, stale lock 1, active lock 2
	// (which then becomes unlocked; but lock 0 also changes to unlocked).
	l.tryLockHook = nil
	if err := ioutil.WriteFile(l.lockFile(0), staleInfo, os.ModePerm); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := ioutil.WriteFile(l.lockFile(1), staleInfo, os.ModePerm); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := ioutil.WriteFile(l.lockFile(2), activeInfo, os.ModePerm); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	assertNumLockFiles(t, d, 3)
	verifyTryLock = goTryLock()
	// TryLock finds level 2 lock held, so it sleeps and retries.
	sleepDuration = <-tk.Requests()
	// Unlock level 2.
	l.unlock(2)
	tryLockCalled = make(chan struct{})
	l.tryLockHook = func() { tryLockCalled <- struct{}{} }
	tk.AdvanceTime(sleepDuration)
	<-tryLockCalled // Back to level 0, TryLock finds it stale.
	// Unlock level 0.
	l.unlock(0)
	<-tryLockCalled // For lock 1.
	<-tryLockCalled // For lock 2.
	// TryLock grabs level 2, but finds level 0 has changed (unlocked).  It
	// sleeps and retries.
	tk.AdvanceTime(<-tk.Requests())
	<-tryLockCalled // Back to level 0.
	// This time, the level 0 lock is found to be available.
	verifyTryLock(true)
	assertNumLockFiles(t, d, 2)
}

type lockState int

const (
	// Free.
	fr lockState = iota
	// Active (held).
	ac
	// Stale.
	st
)

// TestLockLevelConfigurations verifies the behavior of TryLock when faced with
// a variety of initial lock level states and changes to the states while
// TryLock sleeps.
func TestLockLevelConfigurations(t *testing.T) {
	d, l := setup(t)
	defer os.RemoveAll(d)
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	// Generate a stale lock info and an active lock info file.
	if out := sh.FuncCmd(goshTryLock, d).Stdout(); out != "grabbed\n" {
		t.Fatalf("Unexpected output: %s", out)
	}
	staleInfo, err := l.readLockInfo(0)
	if err != nil {
		t.Fatalf("readLockInfo failed: %v", err)
	}
	if err := l.Unlock(); err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}
	if grabbed, err := l.TryLock(); err != nil || !grabbed {
		t.Fatalf("Expected lock was grabbed, got (%t, %v) instead", grabbed, err)
	}
	activeInfo, err := l.readLockInfo(0)
	if err != nil {
		t.Fatalf("readLockInfo failed: %v", err)
	}
	if err := l.Unlock(); err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}

	for i, c := range []struct {
		// locks lists the initial state of the locks, subsequent states
		// of the locks to be set while TryLock sleeps, and the final
		// expected state of the locks.
		locks [][]lockState
		// The expected return value of TryLock.
		outcome bool
	}{
		// No existing locks -> success.
		{[][]lockState{
			[]lockState{},
			[]lockState{ac},
		}, true},
		// Level 0 held -> can't lock.
		{[][]lockState{
			[]lockState{ac},
			[]lockState{ac},
		}, false},
		// Level 0 stale -> success.
		{[][]lockState{
			[]lockState{st},
			[]lockState{ac},
		}, true},
		// Level 0-4 stale -> success.
		{[][]lockState{
			[]lockState{st, st, st, st, st},
			[]lockState{ac},
		}, true},
		// Scenarios where TryLock sleeps, during which time we alter
		// the lock states.
		{[][]lockState{
			[]lockState{st, st, ac, st, st},
			[]lockState{st, st, st, st, st},
			[]lockState{ac},
		}, true},
		{[][]lockState{
			[]lockState{st, st, ac, st, st},
			[]lockState{st, st, st, st, st},
			[]lockState{ac},
		}, true},
		{[][]lockState{
			[]lockState{st, st, ac, st, st},
			[]lockState{st, st, st, st, ac},
			[]lockState{st, st, st, ac, st},
			[]lockState{ac, st, st, ac, st},
			[]lockState{ac, st, st, ac, st},
		}, false},
		{[][]lockState{
			[]lockState{st, st, ac, st, st, fr, st, ac},
			[]lockState{st, st, st, st, st, fr, fr, ac},
			[]lockState{ac, fr, fr, fr, fr, fr, fr, ac},
		}, true},
		{[][]lockState{
			[]lockState{st, st, ac},
			[]lockState{st, st, st, st, st},
			[]lockState{ac},
		}, true},
	} {
		setStates := func(states []lockState) {
			// First, remove all lock files.
			files, err := ioutil.ReadDir(d)
			if err != nil {
				t.Fatalf("ReadDir failed: %v", err)
			}
			for _, f := range files {
				os.Remove(filepath.Join(d, f.Name()))
			}
			assertNumLockFiles(t, d, 0)
			// Populate the lock files specified by states.
			for level, state := range states {
				switch state {
				default:
					t.Fatalf("Unexpected state: %v", state)
				case fr:
				case ac:
					if err := ioutil.WriteFile(l.lockFile(level), activeInfo, os.ModePerm); err != nil {
						t.Fatalf("WriteFile failed: %v", err)
					}
				case st:
					if err := ioutil.WriteFile(l.lockFile(level), staleInfo, os.ModePerm); err != nil {
						t.Fatalf("WriteFile failed: %v", err)
					}
				}
			}
		}
		setStates(c.locks[0])
		statesIndex := 1
		// Spin up a goroutine to wake up sleeping locks.
		closeTKLoop := make(chan struct{})
		tkLoopDone := make(chan struct{})
		go func() {
			defer close(tkLoopDone)
			tk := l.timeKeeper.(timekeeper.ManualTime)
			for {
				select {
				case sleepDuration := <-tk.Requests():
					setStates(c.locks[statesIndex])
					statesIndex++
					tk.AdvanceTime(sleepDuration)
				case <-closeTKLoop:
					return
				}
			}
		}()
		if outcome, err := l.TryLock(); err != nil {
			t.Fatalf("case %d: TryLock failed: %v", i, err)
		} else if outcome != c.outcome {
			t.Fatalf("case %d: expected outcome %t, got %t instead", i, c.outcome, outcome)
		}
		close(closeTKLoop)
		<-tkLoopDone
		if statesIndex != len(c.locks)-1 {
			t.Fatalf("case %d: expected to exhaust lock states, but statesIndex is %d", i, statesIndex)
		}
		for level, state := range c.locks[statesIndex] {
			switch state {
			default:
				t.Fatalf("case %d: Unexpected state: %v", i, state)
			case fr:
				verifyFreeLock(t, l, level)
			case ac:
				verifyHeldLock(t, l, level)
				l.unlock(level)
			case st:
				verifyStaleLock(t, l, level)
				l.unlock(level)
			}
		}
		assertNumLockFiles(t, d, 0)
	}
}

// TestStaleLockMany verifies it's ok to have many concurrent lockers trying to
// grab the same stale lock.  Only one should succeed at a time.  This is a
// black-box test (if we ignore the timeKeeper manipulation).
func TestStaleLockMany(t *testing.T) {
	d, l := setup(t)
	defer os.RemoveAll(d)
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	// Make a stale lock.
	if out := sh.FuncCmd(goshTryLock, d).Stdout(); out != "grabbed\n" {
		t.Fatalf("Unexpected output: %s", out)
	}

	// Spin up a goroutine to wake up sleeping locks.
	closeTKLoop := make(chan struct{})
	tkLoopDone := make(chan struct{})
	go func() {
		defer close(tkLoopDone)
		tk := l.timeKeeper.(timekeeper.ManualTime)
		for {
			select {
			case sleepDuration := <-tk.Requests():
				tk.AdvanceTime(sleepDuration)
			case <-closeTKLoop:
				return
			}
		}
	}()

	// Bring up a bunch of lockers at the same time, contending over the
	// same stale lock.
	const nLockers = 50
	errors := make(chan error, 2*nLockers)
	for i := 0; i < nLockers; i++ {
		go func() {
			errors <- l.Lock()
			// If more than one locker would be allowed in here,
			// Unlock will return an error for the second attempt at
			// freeing the lock.
			errors <- l.Unlock()
		}()
	}
	for i := 0; i < 2*nLockers; i++ {
		if err := <-errors; err != nil {
			t.Fatalf("Error (%d): %v", i, err)
		}
	}
	close(closeTKLoop)
	<-tkLoopDone
}

func TestMain(m *testing.M) {
	gosh.InitMain()
	os.Exit(m.Run())
}

// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package lock provides a lock object to synchronize access to a directory
// among multiple processes.
//
// The implementation relies on lock files written inside the locked directory.
//
// To safely handle situations where a process holding the lock exits without
// releasing the lock, the implementation employs a multi-level locking
// strategy: lock level 0 is considered the master lock.  If the master lock is
// found to be stale (the owning process is dead), the process seeking the lock
// will need to grab lock level 1 before it can re-claim lock level 0 (only one
// process can do so, ensuring only one process proceeds to own lock level 0).
//
// If in turn lock level 1 is found to be stale, the process will need to grab
// lock level 2, and so on.  Eventually, the process seeking the lock will reach
// a level N where the lock is not stale: if the level N lock is available, the
// process grabs it, reclaims lock level 0 for itself (after checking it's
// allowed to), and removes all locks for levels > 0.
//
// If the level N lock is held by another process, the process seeking the lock
// will retry starting at level 0.
//
// Lock level 0 determines who holds the lock once TryLock returns.  Higher
// level locks are only held while TryLock is running, and are ephemeral.  This
// scheme ensures that exactly one process ever grabs the lock regardless of
// whether it's stale.
package lock

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"v.io/x/ref/lib/timekeeper"
	"v.io/x/ref/services/agent/internal/lockutil"
)

// TryLocker is a sync.Locker augmented with TryLock.
type TryLocker interface {
	sync.Locker
	// TryLock attempts to grab the lock, but does not hang if the lock is
	// actively held by another process.  Instead, it returns false.
	TryLock() bool
}

// TryLockerSafe is like TryLocker, but the methods can return an error and
// never panic.
type TryLockerSafe interface {
	// TryLock attempts to grab the lock, but does not hang if the lock is
	// actively held by another process.  Instead, it returns false.
	TryLock() (bool, error)
	// Lock blocks until it's able to grab the lock.
	Lock() error
	// Unlock releases the lock.  Should only be called when the lock is
	// held.
	Unlock() error
	// Must returns a TryLocker whose Lock, TryLock and Unlock methods
	// panic rather than return errors.
	Must() TryLocker
}

const (
	lockFileName = "lock"
	tempFileName = "templock"
	maxTries     = 50
	maxIndex     = 20
	sleepTime    = 100 * time.Millisecond
	sleepJitter  = 20 * time.Millisecond // Should be < sleepTime.
)

// NewDirLock creates a new TryLockerSafe that can be used to manipulate a file
// lock for a directory.
func NewDirLock(dir string) TryLockerSafe {
	return newDirLock(dir, timekeeper.RealTime())
}

type dirLock struct {
	// dir is the directory being locked.
	dir string
	// timeKeeper allows injection of controlled time for testing.
	timeKeeper timekeeper.TimeKeeper
	// tryLockHook allows injection of test logic at the end of tryLock.
	tryLockHook func()
	// randomizer is used to jitter sleep time. Guarded by randMutex.
	randomizer *rand.Rand
	// randMutex guards against concurrent access to randomizer.
	randMutex sync.Mutex
}

func newDirLock(dir string, t timekeeper.TimeKeeper) *dirLock {
	return &dirLock{
		dir:        dir,
		timeKeeper: t,
		randomizer: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// lockFile returns the path to the lock file for level #index.
func (l *dirLock) lockFile(index int) string {
	return filepath.Join(l.dir, lockFileName+strconv.Itoa(index))
}

// readLockInfo returns the lock process information for lock level #index.
func (l *dirLock) readLockInfo(index int) ([]byte, error) {
	return ioutil.ReadFile(l.lockFile(index))
}

type lockOutcome interface {
	grabbed() bool
}

type grabbedLock bool

func (gl grabbedLock) grabbed() bool {
	return bool(gl)
}

type staleLock string

func (staleLock) grabbed() bool {
	return false
}

type failedLock struct {
	error
}

func (failedLock) grabbed() bool {
	return false
}

// tryLock tries to grab lock level #index.
func (l *dirLock) tryLock(index int) lockOutcome {
	if l.tryLockHook != nil {
		defer l.tryLockHook()
	}
	tmpFile, err := lockutil.CreateLockFile(l.dir, tempFileName)
	if err != nil {
		return failedLock{err}
	}
	defer os.Remove(tmpFile)
	if err = os.Link(tmpFile, l.lockFile(index)); err == nil {
		// Successfully grabbed the lock.
		return grabbedLock(true)
	}
	if linkErr, ok := err.(*os.LinkError); !ok || !os.IsExist(linkErr.Err) {
		// Some unexpected error occured.
		return failedLock{err}
	}
	// Lock is either actively held, or stale.  Figure out which.
	lockInfo, err := l.readLockInfo(index)
	if err != nil {
		if os.IsNotExist(err) {
			// Lock was just released.  Count that as already held.
			return grabbedLock(false)
		}
		return failedLock{err}
	}
	if running, err := lockutil.StillHeld(lockInfo); err != nil {
		return failedLock{err}
	} else if running {
		// Lock is held by someone else.
		return grabbedLock(false)
	}
	// Lock appears to be stale.
	return staleLock(lockInfo)
}

// poachLock grabs lock level #index forcefully.  Meant to be called when we're
// entitled to own the lock.
func (l *dirLock) poachLock(index int) error {
	tmpFile, err := lockutil.CreateLockFile(l.dir, tempFileName)
	if err != nil {
		return nil
	}
	defer os.Remove(tmpFile)
	return os.Rename(tmpFile, l.lockFile(index))
}

// unlock removes lock level #index.  Meant to be called when we own the lock.
func (l *dirLock) unlock(index int) error {
	return os.Remove(l.lockFile(index))
}

func (l *dirLock) sleep() {
	// randomize sleep time to avoid sleep/wake cycle alignment among
	// processes contending for the same lock.
	l.randMutex.Lock()
	sleepDuration := sleepTime + time.Duration(l.randomizer.Int63n(int64(sleepJitter)))
	l.randMutex.Unlock()
	l.timeKeeper.Sleep(sleepDuration)
}

// TryLock implements TryLockerSafe.TryLock.
func (l *dirLock) TryLock() (bool, error) {
retry:
	for tries := 0; tries < maxTries; tries++ {
		var masterLockInfo []byte
		// Lock level #0 is the master lock.
		switch t := l.tryLock(0).(type) {
		default:
			return false, fmt.Errorf("unexpected type: %T", t)
		case failedLock:
			return false, t.error
		case grabbedLock:
			// Either we grabbed the master lock, or it's being
			// legitimately held by another process.
			return bool(t), nil
		case staleLock:
			// Master lock appears to be stale.  Proceed to grab the
			// next level lock.
			masterLockInfo = []byte(t)
		}

		// Go up the levels, trying to find the first non-stale lock.
		for index := 1; index < maxIndex; index++ {
			switch t := l.tryLock(index).(type) {
			default:
				return false, fmt.Errorf("unexpected type: %T", t)
			case failedLock:
				return false, t.error
			case grabbedLock:
				if !bool(t) {
					// The lock is being legitimately held
					// by some other process.  Retry from
					// level 0 after some delay.
					l.sleep()
					continue retry
				}
				// We grabbed lock level #index.  Before taking
				// over the master lock, we need to make sure
				// the master lock has not changed since we
				// examined it (this ensures that it's safe for
				// the legitimate owner of the master lock to
				// remove all higher level locks).
				currMasterLockInfo, err := l.readLockInfo(0)
				if err != nil && !os.IsNotExist(err) {
					l.unlock(index)
					return false, err
				}
				if err != nil || !bytes.Equal(currMasterLockInfo, masterLockInfo) {
					// The master lock has changed.  We are
					// not the rightful owner.  Retry from
					// level 0 after some delay.
					l.unlock(index)
					l.sleep()
					continue retry
				}
			case staleLock:
				// Lock appears to be stale.  Proceed to the
				// next level lock.
				continue
			}
			// We grabbed the lock and are the rightful owner.
			// Steal the master lock and remove higher level locks.
			if err := l.poachLock(0); err != nil {
				l.unlock(index)
				return false, err
			}
			// It's now safe to clear higher level locks since any
			// other processes waiting to grab higher level locks
			// will have to resume from level 0 once they notice
			// that the master lock info has changed.
			for i := index; i > 0; i-- {
				l.unlock(i)
			}
			return true, nil
		}
		return false, fmt.Errorf("max number of lock levels exceeded: %d", maxIndex)
	}
	return false, fmt.Errorf("max number of tries exceeded: %d", maxTries)
}

// Lock implements TryLockerSafe.Lock.
func (l *dirLock) Lock() error {
	for {
		if locked, err := l.TryLock(); err != nil {
			return err
		} else if locked {
			return nil
		}
		// Lock is held by another (live) process.  Retry after an
		// appropriate delay.
		l.sleep()
	}
}

// Unlock implements TryLockerSafe.Unlock.
func (l *dirLock) Unlock() error {
	return l.unlock(0)
}

type mustLock struct {
	l TryLockerSafe
}

// Lock implements sync.Locker.Lock.
func (m *mustLock) Lock() {
	if err := m.l.Lock(); err != nil {
		panic(err)
	}
}

// TryLock implements TryLocker.TryLock.
func (m *mustLock) TryLock() bool {
	got, err := m.l.TryLock()
	if err != nil {
		panic(err)
	}
	return got
}

// Unlock implements sync.Locker.Unlock.
func (m *mustLock) Unlock() {
	if err := m.l.Unlock(); err != nil {
		panic(err)
	}
}

// Must implements TryLockerSafe.Must.
func (l *dirLock) Must() TryLocker {
	return &mustLock{l}
}

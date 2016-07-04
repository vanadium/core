// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package watchable

import (
	"fmt"
	"strconv"
	"sync"

	"v.io/v23/services/watch"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/syncbase/common"
	"v.io/x/ref/services/syncbase/store"
)

// watcher maintains a set of watch clients receiving on update channels. When
// watcher is notified of a change in the store, it sends a value to all client
// channels that do not already have a value pending. This is done by a separate
// goroutine (watcherLoop) to move it off the broadcastUpdates() critical path.
type watcher struct {
	// Channel used by broadcastUpdates() to notify watcherLoop. When watcher is
	// closed, updater is closed.
	updater chan struct{}
	// Protects the clients map.
	mu sync.RWMutex
	// Channels used by watcherLoop to notify currently registered clients. When
	// watcher is closed, all client channels are closed and clients is set to nil.
	clients map[chan struct{}]struct{}
}

func newWatcher() *watcher {
	ret := &watcher{
		updater: make(chan struct{}, 1),
		clients: make(map[chan struct{}]struct{}),
	}
	go ret.watcherLoop()
	return ret
}

// close closes the watcher. Idempotent.
func (w *watcher) close() {
	w.mu.Lock()
	if w.clients != nil {
		// Close all client channels.
		for c := range w.clients {
			closeAndDrain(c)
		}
		// Set clients to nil to mark watcher as closed.
		w.clients = nil
		// Close updater to notify watcherLoop to exit.
		closeAndDrain(w.updater)
	}
	w.mu.Unlock()
}

// broadcastUpdates notifies the watcher of an update. The watcher loop will
// propagate the notification to watch clients.
func (w *watcher) broadcastUpdates() {
	w.mu.RLock()
	if w.clients != nil {
		ping(w.updater)
	} else {
		vlog.Error("broadcastUpdates() called on a closed watcher")
	}
	w.mu.RUnlock()
}

// watcherLoop implements the goroutine that waits for updates and notifies any
// waiting clients.
func (w *watcher) watcherLoop() {
	for {
		// If updater has been closed, exit.
		if _, ok := <-w.updater; !ok {
			return
		}
		w.mu.RLock()
		for c := range w.clients { // safe for w.clients == nil
			ping(c)
		}
		w.mu.RUnlock()
	}
}

// ping writes a signal to a buffered notification channel. If a notification
// is already pending, it is a no-op.
func ping(c chan<- struct{}) {
	select {
	case c <- struct{}{}: // sent notification
	default: // already has notification pending
	}
}

// closeAndDrain closes a buffered notification channel and drains the buffer
// so that receivers see the closed state sooner.
func closeAndDrain(c chan struct{}) {
	close(c)
	for _, ok := <-c; ok; _, ok = <-c {
		// no-op
	}
}

// watchUpdates - see WatchUpdates.
func (w *watcher) watchUpdates() (update <-chan struct{}, cancel func()) {
	updateRW := make(chan struct{}, 1)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.clients == nil {
		// watcher is closed, return a closed update channel and no-op cancel.
		close(updateRW)
		cancel = func() {}
		return updateRW, cancel
	}
	// Register update channel.
	w.clients[updateRW] = struct{}{}
	// Cancel is idempotent. It unregisters and closes the update channel.
	cancel = func() {
		w.mu.Lock()
		if _, ok := w.clients[updateRW]; ok { // safe for w.clients == nil
			delete(w.clients, updateRW)
			closeAndDrain(updateRW)
		}
		w.mu.Unlock()
	}
	return updateRW, cancel
}

// WatchUpdates returns a channel that can be used to wait for changes of the
// database, as well as a cancel function which MUST be called to release the
// watch resources. If the update channel is closed, the store is closed and
// no more updates will happen. Otherwise, the channel will have a value
// available whenever the store has changed since the last receive on the
// channel.
func WatchUpdates(st store.Store) (update <-chan struct{}, cancel func()) {
	// TODO(rogulenko): Remove dynamic type assertion here and in other places.
	watcher := st.(*Store).watcher
	return watcher.watchUpdates()
}

// GetResumeMarker returns the ResumeMarker that points to the current end
// of the event log.
func GetResumeMarker(st store.StoreReader) (watch.ResumeMarker, error) {
	seq, err := getNextLogSeq(st)
	return watch.ResumeMarker(logEntryKey(seq)), err
}

// MakeResumeMarker converts a sequence number to the resume marker.
func MakeResumeMarker(seq uint64) watch.ResumeMarker {
	return watch.ResumeMarker(logEntryKey(seq))
}

func logEntryKey(seq uint64) string {
	// Note: MaxUint64 is 0xffffffffffffffff.
	// TODO(sadovsky): Use a more space-efficient lexicographic number encoding.
	return join(common.LogPrefix, fmt.Sprintf("%016x", seq))
}

// ReadBatchFromLog returns a batch of watch log records (a transaction) from
// the given database and the new resume marker at the end of the batch.
func ReadBatchFromLog(st store.Store, resumeMarker watch.ResumeMarker) ([]*LogEntry, watch.ResumeMarker, error) {
	seq, err := parseResumeMarker(string(resumeMarker))
	if err != nil {
		return nil, resumeMarker, err
	}
	_, scanLimit := common.ScanPrefixArgs(common.LogPrefix, "")
	scanStart := resumeMarker
	endOfBatch := false

	// Use the store directly to scan these read-only log entries, no need
	// to create a snapshot since they are never overwritten.  Read and
	// buffer a batch before processing it.
	var logs []*LogEntry
	stream := st.Scan(scanStart, scanLimit)
	for stream.Advance() {
		seq++
		var logEnt LogEntry
		if err := vom.Decode(stream.Value(nil), &logEnt); err != nil {
			stream.Cancel()
			return nil, resumeMarker, err
		}

		logs = append(logs, &logEnt)

		// Stop if this is the end of the batch.
		if logEnt.Continued == false {
			endOfBatch = true
			stream.Cancel()
			break
		}
	}

	if !endOfBatch {
		if err = stream.Err(); err != nil {
			return nil, resumeMarker, err
		}
		if len(logs) > 0 {
			vlog.Fatalf("end of batch not found after %d entries", len(logs))
		}
		return nil, resumeMarker, nil
	}
	return logs, watch.ResumeMarker(logEntryKey(seq)), nil
}

func parseResumeMarker(resumeMarker string) (uint64, error) {
	// See logEntryKey() for the structure of a resume marker key.
	parts := common.SplitNKeyParts(resumeMarker, 2)
	if len(parts) != 2 {
		return 0, verror.New(watch.ErrUnknownResumeMarker, nil, resumeMarker)
	}
	seq, err := strconv.ParseUint(parts[1], 16, 64)
	if err != nil {
		return 0, verror.New(watch.ErrUnknownResumeMarker, nil, resumeMarker)
	}
	return seq, nil
}

// logEntryExists returns true iff the log contains an entry with the given
// sequence number.
func logEntryExists(st store.StoreReader, seq uint64) (bool, error) {
	_, err := st.Get([]byte(logEntryKey(seq)), nil)
	if err != nil && verror.ErrorID(err) != store.ErrUnknownKey.ID {
		return false, err
	}
	return err == nil, nil
}

// getNextLogSeq returns the next sequence number to be used for a new commit.
// NOTE: this function assumes that all sequence numbers in the log represent
// some range [start, limit] without gaps.
func getNextLogSeq(st store.StoreReader) (uint64, error) {
	// Determine initial value for seq.
	// TODO(sadovsky): Consider using a bigger seq.

	// Find the beginning of the log.
	it := st.Scan(common.ScanPrefixArgs(common.LogPrefix, ""))
	if !it.Advance() {
		return 0, nil
	}
	defer it.Cancel()
	if it.Err() != nil {
		return 0, it.Err()
	}
	seq, err := parseResumeMarker(string(it.Key(nil)))
	if err != nil {
		return 0, err
	}
	var step uint64 = 1
	// Suppose the actual value we are looking for is S. First, we estimate the
	// range for S. We find seq, step: seq < S <= seq + step.
	for {
		if ok, err := logEntryExists(st, seq+step); err != nil {
			return 0, err
		} else if !ok {
			break
		}
		seq += step
		step *= 2
	}
	// Next we keep the seq < S <= seq + step invariant, reducing step to 1.
	for step > 1 {
		step /= 2
		if ok, err := logEntryExists(st, seq+step); err != nil {
			return 0, err
		} else if ok {
			seq += step
		}
	}
	// Now seq < S <= seq + 1, thus S = seq + 1.
	return seq + 1, nil
}

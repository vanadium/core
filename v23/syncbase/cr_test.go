// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase/crtestutil"
	"v.io/v23/vom"
)

// TODO(jlodhia): Extract these tests (along with the crtestutil package code)
// to syncbase_test, or alternatively, move crtestutil to an internal dir.

////////////////////////////////////////
// Client Code

// Client constants
const (
	TaskIdPrefix = "task:"
	ListIdPrefix = "list:"

	List1 = "list:1"

	Task1 = "task:1:1"
	Task2 = "task:1:2"

	HintListDelete = "ListDelete"

	HintTextEdit   = "TaskTextEdit"
	HintDone       = "TaskDoneToggle"
	HintTaskDelete = "TaskDelete"
	HintTaskAdd    = "TaskAdd"

	TextNew = "EditedText"
)

// Client data structures
type Task struct {
	Text   string
	Done   bool
	DoneTs int64
	ListId string
}

type List struct {
	Name      string
	TaskCount int16
	DoneCount int16
}

// Client conflict reolution impl
type ConflictResolverImpl struct {
}

func (ri *ConflictResolverImpl) OnConflict(ctx *context.T, conflict *Conflict) Resolution {
	taskRows := findRowsByType(conflict, TaskIdPrefix)
	result := ri.handleTasks(ctx, taskRows)

	listRows := findRowsByType(conflict, ListIdPrefix)
	ri.handleLists(ctx, listRows, result)
	return Resolution{ResultSet: result}
}

// handleTasks does a merge of fields within each task under conflict.
// It handles conflict on fields as follows:
// Text: Last writer wins
// Done: Depends on DoneTs
// DoneTs: The side that is not equal to ancestor and is the lower of the two
//         wins. This means that if a user marks a task done on one device
//         and then later marks the same task done on another device that has
//         not synced yet, the done timestamp that is selected will be from the
//         former action.
func (ri *ConflictResolverImpl) handleTasks(ctx *context.T, tasks []ConflictRow) map[string]ResolvedRow {
	result := map[string]ResolvedRow{}
	for _, t := range tasks {
		if isRowAdded(t) {
			result[t.Key] = handleRowAdd(t)
			continue
		}
		if isRowDeleted(t) {
			result[t.Key] = ResolvedRow{Key: t.Key, Result: nil}
			continue
		}

		var localTask, remoteTask, ancestorTask, resolvedTask Task
		t.LocalValue.Get(&localTask)
		t.RemoteValue.Get(&remoteTask)
		t.AncestorValue.Get(&ancestorTask)

		// prime the reolved task with local version for simplicity.
		resolvedTask = localTask

		// check field text for conflict
		if localTask.Text != remoteTask.Text {
			if remoteTask.Text != ancestorTask.Text {
				if (localTask.Text == ancestorTask.Text) || (t.RemoteValue.WriteTs.After(t.LocalValue.WriteTs)) {
					resolvedTask.Text = remoteTask.Text
				}
			}
		}

		// check field done for conflict
		if localTask.DoneTs != remoteTask.DoneTs {
			if remoteTask.DoneTs != ancestorTask.DoneTs {
				if (localTask.DoneTs == ancestorTask.DoneTs) || (remoteTask.DoneTs < localTask.DoneTs) {
					resolvedTask.DoneTs = remoteTask.DoneTs
					resolvedTask.Done = remoteTask.Done
				}
			}
		}
		resValue, _ := NewValue(ctx, &resolvedTask)
		result[t.Key] = ResolvedRow{Key: t.Key, Result: resValue}
	}
	return result
}

// handleLists does a merge of fields within each list under conflict. If a list
// is deleted, it looks at the resolution map, finds all tasks that belong to
// the list and changes the resolution to delete for those tasks.
// It handles conflict on fields as follows:
// Name: Last writer wins
// TaskCount: Add both counts and subtract the ancestor count.
// DoneCount: Add both counts and subtract the ancestor count.
// NOTE: The above counters are resolved in a naive way for simplicity of this
// test. There are many other ways to represent the counters in a more correct
// way.
func (ri *ConflictResolverImpl) handleLists(ctx *context.T, lists []ConflictRow, resolution map[string]ResolvedRow) {
	for _, l := range lists {
		if isRowAdded(l) {
			resolution[l.Key] = handleRowAdd(l)
			continue
		}
		if isRowDeleted(l) {
			resolution[l.Key] = ResolvedRow{Key: l.Key, Result: nil}
			deleteTasksForList(resolution, l.Key)
			continue
		}

		if l.LocalValue.State == wire.ValueStateUnknown {
			resolution[l.Key] = ResolvedRow{Key: l.Key, Result: &l.RemoteValue}
			continue
		}
		if l.RemoteValue.State == wire.ValueStateUnknown {
			resolution[l.Key] = ResolvedRow{Key: l.Key, Result: &l.LocalValue}
			continue
		}

		var localList, remoteList, ancestorList, resolvedList List
		l.LocalValue.Get(&localList)
		l.RemoteValue.Get(&remoteList)
		l.AncestorValue.Get(&ancestorList)

		// prime the resolved task with local version for simplicity.
		resolvedList = localList

		// check field Name for conflict
		if localList.Name != remoteList.Name {
			if remoteList.Name != ancestorList.Name {
				if (localList.Name == ancestorList.Name) || (l.RemoteValue.WriteTs.After(l.LocalValue.WriteTs)) {
					resolvedList.Name = remoteList.Name
				}
			}
		}

		// check field TaskCount for conflict
		if localList.TaskCount != remoteList.TaskCount {
			if remoteList.TaskCount != ancestorList.TaskCount {
				if localList.TaskCount == ancestorList.TaskCount {
					resolvedList.TaskCount = remoteList.TaskCount
				} else {
					resolvedList.TaskCount = localList.TaskCount + remoteList.TaskCount - ancestorList.TaskCount
				}
			}
		}

		// check field DoneCount for conflict
		if localList.DoneCount != remoteList.DoneCount {
			if remoteList.DoneCount != ancestorList.DoneCount {
				if localList.DoneCount == ancestorList.DoneCount {
					resolvedList.DoneCount = remoteList.DoneCount
				} else {
					resolvedList.DoneCount = localList.DoneCount + remoteList.DoneCount - ancestorList.DoneCount
				}
			}
		}
		resValue, _ := NewValue(ctx, &resolvedList)
		resolution[l.Key] = ResolvedRow{Key: l.Key, Result: resValue}
	}
}

func deleteTasksForList(resolution map[string]ResolvedRow, listKey string) {
	// find key prefix for tasks belonging to list
	taskKeyPrefix := strings.Replace(listKey, ListIdPrefix, TaskIdPrefix, 1) + ":"
	for k, v := range resolution {
		if strings.HasPrefix(k, taskKeyPrefix) {
			v.Result = nil
			resolution[k] = v
		}
	}
}

func handleRowAdd(r ConflictRow) ResolvedRow {
	var resolvedTask Value
	// first one to add wins
	if r.RemoteValue.State != wire.ValueStateExists {
		resolvedTask = r.LocalValue
	} else if r.LocalValue.State != wire.ValueStateExists {
		resolvedTask = r.RemoteValue
	} else if r.LocalValue.WriteTs.Before(r.RemoteValue.WriteTs) {
		resolvedTask = r.LocalValue
	} else {
		resolvedTask = r.RemoteValue
	}
	return ResolvedRow{Key: r.Key, Result: &resolvedTask}
}

func isRowDeleted(r ConflictRow) bool {
	return r.LocalValue.State == wire.ValueStateDeleted ||
		r.RemoteValue.State == wire.ValueStateDeleted
}

func isRowAdded(r ConflictRow) bool {
	return r.AncestorValue.State == wire.ValueStateNoExists &&
		(r.LocalValue.State == wire.ValueStateExists || r.RemoteValue.State == wire.ValueStateExists)
}

func findRowsByType(conflict *Conflict, typePrefix string) []ConflictRow {
	rows := []ConflictRow{}
	if conflict.WriteSet != nil {
		for k, v := range conflict.WriteSet.ByKey {
			if strings.HasPrefix(k, typePrefix) {
				rows = append(rows, v)
			}
		}
	}
	return rows
}

////////////////////////////////////////
// Tests

func TestConflictResolutionSingleObject(t *testing.T) {
	mockConflict, expResult := simpleConflictStream(t)
	RunTest(t, mockConflict, expResult, false)
}

func TestConflictResolutionAddDelete(t *testing.T) {
	mockConflict, expResult := addDeleteConflictStream(t)
	RunTest(t, mockConflict, expResult, true)
}

func TestConflictResolutionIntersectingConflicts(t *testing.T) {
	mockConflict, expResult := twoIntersectingBatchesConflictStream(t)
	RunTest(t, mockConflict, expResult, true)
}

func TestConflictResolutionListDelete(t *testing.T) {
	mockConflict, expResult := listDeleteConflictsWithTaskAdd(t)
	RunTest(t, mockConflict, expResult, true)
}

func RunTest(t *testing.T, mockConflict []wire.ConflictInfo, expResult map[string]wire.ResolutionInfo, singleBatch bool) {
	db := newDatabase("parentName", wire.Id{"a1", "db1"}, getSchema(&ConflictResolverImpl{}))
	advance := func(st *crtestutil.State) bool {
		if st.ValIndex >= len(mockConflict) {
			st.SetIsBlocked(true)
			// Wait for test to process all conflicts.
			st.Mu.Lock()
			defer st.Mu.Unlock()
			return false
		}
		st.SetIsBlocked(false)
		st.Val = mockConflict[st.ValIndex]
		st.ValIndex++
		return true
	}

	st := &crtestutil.State{}
	crStream := &crtestutil.CrStreamImpl{
		C: &crtestutil.ConflictStreamImpl{St: st, AdvanceFn: advance},
		R: &crtestutil.ResolutionStreamImpl{St: st},
	}
	db.c = crtestutil.MockDbClient(db.c, crStream)
	db.crState.reconnectWaitTime = 10 * time.Millisecond

	st.Mu.Lock() // causes Advance() to block
	defer st.Mu.Unlock()

	ctx, cancel := context.RootContext()
	db.EnforceSchema(ctx)
	defer cancel()
	defer db.Close()
	for i := 0; i < 100 && (len(st.GetResult()) != len(expResult)); i++ {
		time.Sleep(time.Millisecond) // wait till Advance() call is blocked
	}
	if len(st.GetResult()) != len(expResult) {
		t.Errorf("\n Unexpected number of results. Expected: %d, actual: %d", len(expResult), len(st.GetResult()))
	}
	for i, result := range st.GetResult() {
		compareResult(t, expResult[result.Key], result)
		if singleBatch {
			if result.Continued != (i < len(st.GetResult())-1) {
				t.Error("\nUnexpected value for continued in single batch")
			}
		} else {
			if result.Continued != false {
				t.Error("\nUnexpected value for continued in multi batch")
			}
		}
	}
}

////////////////////////////////////////
// Test helpers to create test data

// Consider a database with a list List1 with two tasks Task1, Task2. The
// following helper methods create conflicts with these three objects in various
// ways.

// simpleConflictStream prepares a conflict stream with two conflicts, one
// each for Task1 and Task2. No batch is involved and Task1 is independent of
// Task2. Assume that List does not get updated for these task updates.
// Local syncbase changes Text field for Task1 and marks Task2 done.
// Remote syncbase marks Task1 done and Task2 done.
// Expected result is a merge for Task1 and Task2.
func simpleConflictStream(t *testing.T) ([]wire.ConflictInfo, map[string]wire.ResolutionInfo) {
	// Conflict for task1
	row1 := makeRowInfo(Task1,
		&wire.Value{Bytes: encode(&Task{"OriginalText", false, -1, List1}), WriteTs: toTime(1)}, // ancestor
		&wire.Value{Bytes: encode(&Task{TextNew, false, -1, List1}), WriteTs: toTime(3)},        // local
		&wire.Value{Bytes: encode(&Task{"OriginalText", true, 20, List1}), WriteTs: toTime(2)})  // remote
	c1 := makeConflictInfo(row1, false)

	// Expected result
	r1 := makeResolution(Task1, encode(&Task{TextNew, true, 20, List1}), wire.ValueSelectionOther)

	// Conflict for task2
	row2 := makeRowInfo(Task2,
		&wire.Value{Bytes: encode(&Task{"Text1", false, -1, List1}), WriteTs: toTime(1)}, // ancestor
		&wire.Value{Bytes: encode(&Task{"Text1", true, 100, List1}), WriteTs: toTime(3)}, // local
		&wire.Value{Bytes: encode(&Task{"Text1", true, 20, List1}), WriteTs: toTime(2)})  // remote
	c2 := makeConflictInfo(row2, false)

	// Expected result
	r2 := makeResolution(Task2, encode(&Task{"Text1", true, 20, List1}),
		wire.ValueSelectionOther) // for simplicity the test resolver does not optimize

	respMap := map[string]wire.ResolutionInfo{}
	respMap[r1.Key] = r1
	respMap[r2.Key] = r2

	return []wire.ConflictInfo{c1, c2}, respMap
}

// addDeleteConflictStream prepares a conflict stream with one conflict
// containing Task1, Task2 and List1. List contains a task count and hence
// addition and deletion of task leads to update to the list.
// Local syncbase deletes Task1 and updates List1 in a batch.
// Remote syncbase adds Task2 and updates List1 in a batch.
// Expected result is Task1 gets deleted, Task2 gets added and List1 shows
// the correct taskcount after the two operations.
func addDeleteConflictStream(t *testing.T) ([]wire.ConflictInfo, map[string]wire.ResolutionInfo) {
	// Batch1 is local and contains task1 and list1
	b1 := makeConflictBatch(1, HintTaskDelete, wire.BatchSourceLocal, true)

	// Batch2 is remote and contains task2 and list1
	b2 := makeConflictBatch(2, HintTaskAdd, wire.BatchSourceRemote, true)

	// Deletion for task1 on local syncbase

	ancestorVal := encode(&Task{"TaskToBeRemoved", false, -1, List1})
	row1 := makeRowInfo(Task1,
		&wire.Value{Bytes: ancestorVal, WriteTs: toTime(1)}, // ancestor
		&wire.Value{State: wire.ValueStateDeleted},          // local
		&wire.Value{State: wire.ValueStateUnknown})          // remote
	row1.BatchIds = []uint64{1}
	c1 := makeConflictInfo(row1, true)

	// Expected result
	r1 := makeResolution(Task1, nil, wire.ValueSelectionOther)

	// Addition for task2 on remote syncbase
	remoteVal := encode(&Task{"AddedTask", false, -1, List1})
	row2 := makeRowInfo(Task2,
		&wire.Value{State: wire.ValueStateNoExists},       // ancestor
		&wire.Value{State: wire.ValueStateUnknown},        // local
		&wire.Value{Bytes: remoteVal, WriteTs: toTime(2)}) // remote
	row2.BatchIds = []uint64{2}
	c2 := makeConflictInfo(row2, true)

	// Expected result
	r2 := makeResolution(Task2, encode(&Task{"AddedTask", false, -1, List1}), wire.ValueSelectionRemote)

	// Update to List on both sided on filed TaskCount
	listName := "Groceries"
	row3 := makeRowInfo(List1,
		&wire.Value{Bytes: encode(&List{listName, 8, 0}), WriteTs: toTime(25)}, // ancestor value
		&wire.Value{Bytes: encode(&List{listName, 7, 0}), WriteTs: toTime(33)}, // local value
		&wire.Value{Bytes: encode(&List{listName, 9, 0}), WriteTs: toTime(44)}) // remote value
	row3.BatchIds = []uint64{1, 2}
	c3 := makeConflictInfo(row3, false)

	// Expected result
	r3 := makeResolution(List1, encode(&List{listName, 8, 0}), wire.ValueSelectionOther)

	respMap := map[string]wire.ResolutionInfo{}
	respMap[r1.Key] = r1
	respMap[r2.Key] = r2
	respMap[r3.Key] = r3

	return []wire.ConflictInfo{b1, b2, c1, c2, c3}, respMap
}

// twoIntersectingBatchesConflictStream prepares a conflict stream with one
// conflict containing Task1, Task2 and List1. Here we assume that List keeps
// a count of done tasks.
// Local syncbase marks Task1 done and updates done count on List1. Later it
// it also marks Task2 done and updates done count on List1.
// Remote syncbase updates field Text on Task1 followed by an update to field
// Text on Task2 (not in a batch).
// Expected result is a merge for Task1 and Task2 and Local version of List1
// being accepted.
func twoIntersectingBatchesConflictStream(t *testing.T) ([]wire.ConflictInfo, map[string]wire.ResolutionInfo) {
	// Batch1 is local and contains task1 and list1
	b1 := makeConflictBatch(1, HintDone, wire.BatchSourceLocal, true)

	// Batch2 is local and contains task2 and list1
	b2 := makeConflictBatch(2, HintDone, wire.BatchSourceLocal, true)

	// Batch3 is remote and contains task1
	b3 := makeConflictBatch(3, HintTextEdit, wire.BatchSourceRemote, true)

	// Batch4 is remote and contains task2
	b4 := makeConflictBatch(4, HintTextEdit, wire.BatchSourceRemote, true)

	// For task1, mark done on local and edit text on remote
	row1 := makeRowInfo(Task1,
		&wire.Value{Bytes: encode(&Task{"TaskOrig", false, -1, List1}), WriteTs: toTime(1)}, // ancestor
		&wire.Value{Bytes: encode(&Task{"TaskOrig", true, 204, List1}), WriteTs: toTime(5)}, // local
		&wire.Value{Bytes: encode(&Task{"TaskEdit", false, -1, List1}), WriteTs: toTime(3)}) // remote
	row1.BatchIds = []uint64{1, 3}
	c1 := makeConflictInfo(row1, true)

	// Expected result
	r1 := makeResolution(Task1, encode(&Task{"TaskEdit", true, 204, List1}), wire.ValueSelectionOther)

	// For task2, mark done on local and edit text on remote
	row2 := makeRowInfo(Task2,
		&wire.Value{Bytes: encode(&Task{"TaskOrig", false, -1, List1}), WriteTs: toTime(2)}, // ancestor
		&wire.Value{Bytes: encode(&Task{"TaskOrig", true, 204, List1}), WriteTs: toTime(5)}, // local
		&wire.Value{Bytes: encode(&Task{"TaskEdit", false, -1, List1}), WriteTs: toTime(3)}) // remote
	row2.BatchIds = []uint64{2, 4}
	c2 := makeConflictInfo(row2, true)

	// Expected result
	r2 := makeResolution(Task2, encode(&Task{"TaskEdit", true, 204, List1}), wire.ValueSelectionOther)

	// Update to List on Local for updating TaskDoneCount
	listName := "Groceries"
	row3 := makeRowInfo(List1,
		&wire.Value{Bytes: encode(&List{listName, 8, 0}), WriteTs: toTime(1)}, // ancestor value
		&wire.Value{Bytes: encode(&List{listName, 8, 2}), WriteTs: toTime(5)}, // local value
		&wire.Value{State: wire.ValueStateUnknown})                            // remote value
	row3.BatchIds = []uint64{1, 2}
	c3 := makeConflictInfo(row3, false)

	// Expected result
	r3 := makeResolution(List1, encode(&List{listName, 8, 2}), wire.ValueSelectionLocal)

	respMap := map[string]wire.ResolutionInfo{}
	respMap[r1.Key] = r1
	respMap[r2.Key] = r2
	respMap[r3.Key] = r3

	return []wire.ConflictInfo{b1, b2, b3, b4, c1, c2, c3}, respMap
}

// listDeleteConflictsWithTaskAdd prepares a conflict stream with one
// conflict containing Task1, Task2 and List1. Assume Task2 does not exist yet.
// Local syncbase deletes List1 and Task1 as a batch.
// Remote syncbase adds Task2 along with an update to List1 as a batch.
// Expected result is deletion of List1, Task1 and Task2.
func listDeleteConflictsWithTaskAdd(t *testing.T) ([]wire.ConflictInfo, map[string]wire.ResolutionInfo) {
	// Batch1 is local and contains task1 and list1
	b1 := makeConflictBatch(1, HintListDelete, wire.BatchSourceLocal, true)

	// Batch2 is remote and contains task2 and list1
	b2 := makeConflictBatch(2, HintTaskAdd, wire.BatchSourceRemote, true)

	// Delition for list1 on local syncbase
	listName := "Groceries"
	row0 := makeRowInfo(List1,
		&wire.Value{Bytes: encode(&List{listName, 1, 0}), WriteTs: toTime(1)}, // ancestor value
		&wire.Value{State: wire.ValueStateDeleted, WriteTs: toTime(5)},        // local value
		&wire.Value{Bytes: encode(&List{listName, 2, 0}), WriteTs: toTime(3)}) // remote value
	row0.BatchIds = []uint64{1, 2}
	c0 := makeConflictInfo(row0, true)

	// Expected result
	r0 := makeResolution(List1, nil, wire.ValueSelectionOther)

	// Deletion for task1 on local syncbase along with list1
	row1 := makeRowInfo(Task1,
		&wire.Value{Bytes: encode(&Task{"TaskToBeRemoved", false, -1, List1}), WriteTs: toTime(1)}, // ancestor
		&wire.Value{State: wire.ValueStateDeleted, WriteTs: toTime(5)},                             // local
		&wire.Value{State: wire.ValueStateUnknown})                                                 // remote
	row1.BatchIds = []uint64{1}
	c1 := makeConflictInfo(row1, true)

	// Expected result
	r1 := makeResolution(Task1, nil, wire.ValueSelectionOther)

	// Addition for task2 on remote syncbase
	row2 := makeRowInfo(Task2,
		&wire.Value{State: wire.ValueStateNoExists},                                          // ancestor value
		&wire.Value{State: wire.ValueStateUnknown},                                           // local value
		&wire.Value{Bytes: encode(&Task{"AddedTask", false, -1, List1}), WriteTs: toTime(2)}) // remote value
	row2.BatchIds = []uint64{2}
	c2 := makeConflictInfo(row2, false)

	// Expected result
	r2 := makeResolution(Task2, nil, wire.ValueSelectionOther)

	respMap := map[string]wire.ResolutionInfo{}
	respMap[r0.Key] = r0
	respMap[r1.Key] = r1
	respMap[r2.Key] = r2

	return []wire.ConflictInfo{b1, b2, c0, c1, c2}, respMap
}

func makeResolution(key string, result *vom.RawBytes, selection wire.ValueSelection) wire.ResolutionInfo {
	r := wire.ResolutionInfo{}
	r.Key = key
	if result != nil {
		r.Result = &wire.Value{
			Bytes:   result,
			WriteTs: time.Time{},
		}
	}
	r.Selection = selection
	return r
}

func makeConflictBatch(id uint64, hint string, source wire.BatchSource, continued bool) wire.ConflictInfo {
	batch := wire.BatchInfo{Id: id, Hint: hint, Source: source}
	return wire.ConflictInfo{Data: wire.ConflictDataBatch{Value: batch}, Continued: continued}
}

func makeConflictInfo(row wire.RowInfo, continued bool) wire.ConflictInfo {
	return wire.ConflictInfo{
		Data:      wire.ConflictDataRow{Value: row},
		Continued: continued,
	}
}

func makeRowInfo(key string, ancestor *wire.Value, local *wire.Value, remote *wire.Value) wire.RowInfo {
	op := wire.RowOp{}
	op.Key = key

	op.AncestorValue = ancestor
	op.LocalValue = local
	op.RemoteValue = remote

	return wire.RowInfo{
		Op: wire.OperationWrite{Value: op},
	}
}

func toTime(unixNanos int64) time.Time {
	return time.Unix(
		unixNanos/1e9, // seconds
		unixNanos%1e9) // nanoseconds
}

func compareResult(t *testing.T, expected wire.ResolutionInfo, actual wire.ResolutionInfo) {
	if actual.Key != expected.Key {
		t.Error("Key does not match")
	}
	if actual.Selection != expected.Selection {
		t.Errorf("Key: %s", expected.Key)
		t.Errorf("Expected selection: %v, actual selection: %v", expected.Selection, actual.Selection)
	}
	if (expected.Result == nil) && (actual.Result != nil) {
		t.Errorf("Key: %s", expected.Key)
		t.Error("Result expected to be nil but found non nil")
	}
	if expected.Result != nil {
		if actual.Result == nil {
			t.Errorf("Key: %s", expected.Key)
			t.Error("Result found nil")
		}
		var err error
		var actualBytes []byte
		var expectedBytes []byte
		if actualBytes, err = vom.Encode(actual.Result.Bytes); err != nil {
			t.Errorf("Key: %s:   encoding error on actual.Result.Bytes", expected.Key)
		}
		if expectedBytes, err = vom.Encode(expected.Result.Bytes); err != nil {
			t.Errorf("Key: %s:   encoding error on expected.Result.Bytes", expected.Key)
		}
		if bytes.Compare(actualBytes, expectedBytes) != 0 {
			t.Errorf("Key: %s", expected.Key)
			t.Error("Result bytes do not match")
			list := &List{}
			actual.Result.Bytes.ToValue(list)
			t.Errorf("Actual list: %#v", list)
		}
	}
}

func encode(value interface{}) *vom.RawBytes {
	v, _ := vom.RawBytesFromValue(value)
	return v
}

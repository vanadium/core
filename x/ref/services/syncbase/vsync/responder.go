// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

import (
	"container/heap"
	"sort"
	"strings"

	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/syncbase/common"
	"v.io/x/ref/services/syncbase/server/interfaces"
	"v.io/x/ref/services/syncbase/store"
	"v.io/x/ref/services/syncbase/store/watchable"
)

// GetDeltas implements the responder side of the GetDeltas RPC.
func (s *syncService) GetDeltas(ctx *context.T, call interfaces.SyncGetDeltasServerCall,
	req interfaces.DeltaReq, initiator string) (interfaces.DeltaFinalResp, error) {

	vlog.VI(2).Infof("sync: GetDeltas: begin: from initiator %s", initiator)
	defer vlog.VI(2).Infof("sync: GetDeltas: end: from initiator %s", initiator)

	var finalResp interfaces.DeltaFinalResp
	rSt, err := newResponderState(ctx, call, s, req, initiator)
	if err != nil {
		return finalResp, err
	}
	err = rSt.sendDeltasPerDatabase(ctx)
	if !rSt.sg {
		// TODO(m3b): It is unclear what to do if this call returns an error.  We would not wish the GetDeltas call to fail.
		finalResp.SgPriorities = make(interfaces.SgPriorities)
		addSyncgroupPriorities(ctx, s.bst, rSt.sgIds, finalResp.SgPriorities)
	}
	return finalResp, err
}

// responderState is state accumulated per Database by the responder during an
// initiation round.
type responderState struct {
	// Parameters from the request.
	dbId     wire.Id
	sgIds    sgSet
	initVecs interfaces.Knowledge
	sg       bool

	call      interfaces.SyncGetDeltasServerCall // Stream handle for the GetDeltas RPC.
	initiator string
	sync      *syncService
	st        *watchable.Store // Database's watchable.Store.

	diff    genRangeVector
	outVecs interfaces.Knowledge
}

func newResponderState(ctx *context.T, call interfaces.SyncGetDeltasServerCall, sync *syncService, req interfaces.DeltaReq, initiator string) (*responderState, error) {
	rSt := &responderState{
		call:      call,
		sync:      sync,
		initiator: initiator,
	}

	switch v := req.(type) {
	case interfaces.DeltaReqData:
		rSt.dbId = v.Value.DbId
		rSt.sgIds = v.Value.SgIds
		rSt.initVecs = v.Value.Gvs

	case interfaces.DeltaReqSgs:
		rSt.sg = true
		rSt.dbId = v.Value.DbId
		rSt.initVecs = v.Value.Gvs
		rSt.sgIds = make(sgSet)
		// Populate the sgids from the initvec.
		for oid := range rSt.initVecs {
			gid, err := sgID(oid)
			if err != nil {
				vlog.Fatalf("sync: newResponderState: invalid syncgroup key %s", oid)
			}
			rSt.sgIds[interfaces.GroupId(gid)] = struct{}{}
		}
	default:
		return nil, verror.New(verror.ErrInternal, ctx, "nil req")
	}
	return rSt, nil
}

// sendDeltasPerDatabase sends to an initiator from the requesting Syncbase all
// the missing generations corresponding to the prefixes requested for this
// Database, and genvectors summarizing the knowledge transferred from the
// responder to the initiator. Note that the responder does not send any
// generations corresponding to the requesting Syncbase even if this responder
// is ahead of the initiator in its knowledge. The responder can be ahead since
// the responder on the requesting Syncbase can communicate generations newer
// than when its initiator began its work.
//
// The responder operates in three phases:
//
// In the first phase, the initiator is checked against the syncgroup ACLs of
// all the syncgroups it is requesting, and only those prefixes that belong to
// allowed syncgroups are carried forward.
//
// In the second phase, for a given set of nested prefixes from the initiator,
// the shortest prefix in that set is extracted. The initiator's genvector for
// this shortest prefix represents the lower bound on its knowledge for the
// entire set of nested prefixes. This genvector (representing the lower bound)
// is diffed with all the responder genvectors corresponding to same or deeper
// prefixes compared to the initiator prefix. This diff produces a bound on the
// missing knowledge. For example, say the initiator is interested in prefixes
// {foo, foobar}, where each prefix is associated with a genvector. Since the
// initiator strictly has as much or more knowledge for prefix "foobar" as it
// has for prefix "foo", "foo"'s genvector is chosen as the lower bound for the
// initiator's knowledge. Similarly, say the responder has knowledge on prefixes
// {f, foobarX, foobarY, bar}. The responder diffs the genvectors for prefixes
// f, foobarX and foobarY with the initiator's genvector to compute a bound on
// missing generations (all responder's prefixes that match "foo". Note that
// since the responder doesn't have a genvector at "foo", its knowledge at "f"
// is applicable to "foo").
//
// Since the second phase outputs an aggressive calculation of missing
// generations containing more generation entries than strictly needed by the
// initiator, in the third phase, each missing generation is sent to the
// initiator only if the initiator is eligible for it and is not aware of
// it. The generations are sent to the initiator in the same order as the
// responder learned them so that the initiator can reconstruct the DAG for the
// objects by learning older nodes first.
func (rSt *responderState) sendDeltasPerDatabase(ctx *context.T) error {
	// TODO(rdaoud): for such vlog.VI() calls where the function name is
	// embedded, consider using a helper function to auto-fill it instead
	// (see http://goo.gl/mEa4L0) but only incur that overhead when the
	// logging level specified is enabled.
	vlog.VI(3).Infof("sync: sendDeltasPerDatabase: recvd %v: sgids %v, genvecs %v, sg %v", rSt.dbId, rSt.sgIds, rSt.initVecs, rSt.sg)

	if !rSt.sync.isDbSyncable(ctx, rSt.dbId) {
		// The database is offline. Skip the db till it becomes syncable again.
		vlog.VI(1).Infof("sync: sendDeltasPerDatabase: database not allowed to sync, skipping sync on db %v", rSt.dbId)
		return interfaces.NewErrDbOffline(ctx, rSt.dbId)
	}

	// Phase 1 of sendDeltas: Authorize the initiator and respond to the
	// caller only for the syncgroups that allow access.
	err := rSt.authorizeAndFilterSyncgroups(ctx)

	// Check error from phase 1.
	if err != nil {
		vlog.VI(4).Infof("sync: sendDeltasPerDatabase: failed authorization, err %v", err)
		return err
	}

	if len(rSt.initVecs) == 0 {
		return verror.New(verror.ErrInternal, ctx, "empty initiator generation vectors")
	}

	// Phase 2 and 3 of sendDeltas: diff contains the bound on the
	// generations missing from the initiator per device.
	if rSt.sg {
		err = rSt.sendSgDeltas(ctx)
	} else {
		err = rSt.sendDataDeltas(ctx)
	}

	return err
}

// authorizeAndFilterSyncgroups authorizes the initiator against the requested
// syncgroups and filters the initiator's prefixes to only include those from
// allowed syncgroups (phase 1 of sendDeltas).
func (rSt *responderState) authorizeAndFilterSyncgroups(ctx *context.T) error {
	var err error
	rSt.st, err = rSt.sync.getDbStore(ctx, nil, rSt.dbId)
	if err != nil {
		return err
	}

	allowedPfxs := make(map[string]struct{})
	for sgid := range rSt.sgIds {
		// Check permissions for the syncgroup.
		var sg *interfaces.Syncgroup
		sg, err = getSyncgroupByGid(ctx, rSt.st, sgid)
		if err == nil {
			err = authorize(ctx, rSt.call.Security(), sg)
		}
		if err == nil {
			if !rSt.sg {
				for _, c := range sg.Spec.Collections {
					allowedPfxs[toCollectionPrefixStr(c)] = struct{}{}
				}
			}
		} else {
			delete(rSt.sgIds, sgid)
			if rSt.sg {
				delete(rSt.initVecs, string(sgid))
			}
			if verror.ErrorID(err) != verror.ErrNoAccess.ID {
				vlog.Errorf("sync: authorizeAndFilterSyncgroups: accessing/authorizing syncgroup failed %v, err %v", sgid, err)
			}
		}
	}

	if rSt.sg {
		return nil
	}

	// Filter the initiator's prefixes to what is allowed.
	for pfx := range rSt.initVecs {
		if _, ok := allowedPfxs[pfx]; ok {
			continue
		}
		allowed := false
		for p := range allowedPfxs {
			if strings.HasPrefix(pfx, p) {
				allowed = true
			}
		}

		if !allowed {
			delete(rSt.initVecs, pfx)
		}
	}
	return nil
}

// sendSgDeltas computes the bound on missing generations, and sends the missing
// log records across all requested syncgroups (phases 2 and 3 of sendDeltas).
func (rSt *responderState) sendSgDeltas(ctx *context.T) error {
	respVecs, _, err := rSt.sync.copyDbGenInfo(ctx, rSt.dbId, rSt.sgIds)
	if err != nil {
		return err
	}

	rSt.outVecs = make(interfaces.Knowledge)

	for sg, initgv := range rSt.initVecs {
		respgv, ok := respVecs[sg]
		if !ok {
			continue
		}
		rSt.diff = make(genRangeVector)
		rSt.diffGenVectors(respgv, initgv)
		rSt.outVecs[sg] = respgv

		if err := rSt.filterAndSendDeltas(ctx, sg); err != nil {
			return err
		}
	}
	return rSt.sendGenVecs(ctx)
}

// sendDataDeltas computes the bound on missing generations across all requested
// prefixes, and sends the missing log records (phases 2 and 3 of sendDeltas).
func (rSt *responderState) sendDataDeltas(ctx *context.T) error {
	// Phase 2 of sendDeltas: Compute the missing generations.
	if err := rSt.computeDataDeltas(ctx); err != nil {
		return err
	}

	// Phase 3 of sendDeltas: Process the diff, filtering out records that
	// are not needed, and send the remainder on the wire ordered.
	if err := rSt.filterAndSendDeltas(ctx, logDataPrefix); err != nil {
		return err
	}
	return rSt.sendGenVecs(ctx)
}

func (rSt *responderState) computeDataDeltas(ctx *context.T) error {
	respVecs, respGen, err := rSt.sync.copyDbGenInfo(ctx, rSt.dbId, nil)
	if err != nil {
		return err
	}
	respPfxs := extractAndSortPrefixes(respVecs)
	initPfxs := extractAndSortPrefixes(rSt.initVecs)

	rSt.outVecs = make(interfaces.Knowledge)
	rSt.diff = make(genRangeVector)
	pfx := initPfxs[0]

	for _, p := range initPfxs {
		if strings.HasPrefix(p, pfx) && p != pfx {
			continue
		}

		// Process this prefix as this is the start of a new set of
		// nested prefixes.
		pfx = p

		// Lower bound on initiator's knowledge for this prefix set.
		initgv := rSt.initVecs[pfx]

		// Find the relevant responder prefixes and add the corresponding knowledge.
		var respgv interfaces.GenVector
		var rpStart string
		for _, rp := range respPfxs {
			if !strings.HasPrefix(rp, pfx) && !strings.HasPrefix(pfx, rp) {
				// No relationship with pfx.
				continue
			}

			if strings.HasPrefix(pfx, rp) {
				// If rp is a prefix of pfx, remember it because
				// it may be a potential starting point for the
				// responder's knowledge. The actual starting
				// point is the deepest prefix where rp is a
				// prefix of pfx.
				//
				// Say the initiator is looking for "foo", and
				// the responder has knowledge for "f" and "fo",
				// the responder's starting point will be the
				// prefix genvector for "fo". Similarly, if the
				// responder has knowledge for "foo", the
				// starting point will be the prefix genvector
				// for "foo".
				rpStart = rp
			} else {
				// If pfx is a prefix of rp, this knowledge must
				// be definitely sent to the initiator. Diff the
				// prefix genvectors to adjust the delta bound and
				// include in outVec.
				respgv = respVecs[rp]
				rSt.diffGenVectors(respgv, initgv)
				rSt.outVecs[rp] = respgv
			}
		}

		// Deal with the starting point.
		if rpStart == "" {
			// No matching prefixes for pfx were found.
			respgv = make(interfaces.GenVector)
			respgv[rSt.sync.id] = respGen
		} else {
			respgv = respVecs[rpStart]
		}
		rSt.diffGenVectors(respgv, initgv)
		rSt.outVecs[pfx] = respgv
	}

	vlog.VI(3).Infof("sync: computeDataDeltas: %v: diff %v, outvecs %v", rSt.dbId, rSt.diff, rSt.outVecs)
	return nil
}

// filterAndSendDeltas filters the computed delta to remove records already
// known by the initiator, and sends the resulting records to the initiator
// (phase 3 of sendDeltas).
func (rSt *responderState) filterAndSendDeltas(ctx *context.T, pfx string) error {
	// TODO(hpucha): Although ok for now to call SendStream once per
	// Database, would like to make this implementation agnostic.
	sender := rSt.call.SendStream()

	// First two phases were successful. So now on to phase 3. We now visit
	// every log record in the generation range as obtained from phase 1 in
	// their log order. We use a heap to incrementally sort the log records
	// as per their position in the log.
	//
	// Init the min heap, one entry per device in the diff.
	mh := make(minHeap, 0, len(rSt.diff))
	for dev, r := range rSt.diff {
		// Do not send generations belonging to the initiator.
		//
		// TODO(hpucha): This needs to be relaxed to handle the case
		// when the initiator has locally destroyed its
		// database/collection, and has rejoined the syncgroup. In this
		// case, it would like its data sent back as well. Perhaps the
		// initiator's request will contain an indication about its
		// status for the responder to distinguish between the 2 cases.
		if syncbaseIdToName(dev) == rSt.initiator {
			continue
		}

		r.cur = r.min
		rec, err := getNextLogRec(ctx, rSt.st, pfx, dev, r)
		if err != nil {
			return err
		}
		if rec != nil {
			mh = append(mh, rec)
		} else {
			delete(rSt.diff, dev)
		}
	}
	heap.Init(&mh)

	// Process the log records in order.
	var initPfxs []string
	if !rSt.sg {
		initPfxs = extractAndSortPrefixes(rSt.initVecs)
	}
	for mh.Len() > 0 {
		rec := heap.Pop(&mh).(*LocalLogRec)

		if rSt.sg || !filterLogRec(rec, rSt.initVecs, initPfxs) {
			// Send on the wire.
			wireRec, err := rSt.makeWireLogRec(ctx, rec)
			if err != nil {
				return err
			}
			sender.Send(interfaces.DeltaRespRec{*wireRec})
		}

		// Add a new record from the same device if not done.
		dev := rec.Metadata.Id
		rec, err := getNextLogRec(ctx, rSt.st, pfx, dev, rSt.diff[dev])
		if err != nil {
			return err
		}
		if rec != nil {
			heap.Push(&mh, rec)
		} else {
			delete(rSt.diff, dev)
		}
	}
	return nil
}

func (rSt *responderState) sendGenVecs(ctx *context.T) error {
	vlog.VI(3).Infof("sync: sendGenVecs: sending genvecs %v", rSt.outVecs)

	sender := rSt.call.SendStream()
	sender.Send(interfaces.DeltaRespGvs{rSt.outVecs})
	return nil
}

// genRange represents a range of generations (min and max inclusive).
type genRange struct {
	min uint64
	max uint64
	cur uint64
}

type genRangeVector map[uint64]*genRange

// diffGenVectors diffs two generation vectors, belonging to the responder and
// the initiator, and updates the range of generations per device known to the
// responder but not known to the initiator. "gens" (generation range) is passed
// in as an input argument so that it can be incrementally updated as the range
// of missing generations grows when different responder prefix genvectors are
// used to compute the diff.
//
// For example: Generation vector for responder is say RVec = {A:10, B:5, C:1},
// Generation vector for initiator is say IVec = {A:5, B:10, D:2}. Diffing these
// two vectors returns: {A:[6-10], C:[1-1]}.
//
// TODO(hpucha): Add reclaimVec for GCing.
func (rSt *responderState) diffGenVectors(respVec, initVec interfaces.GenVector) {
	// Compute missing generations for devices that are in both initiator's
	// and responder's vectors.
	for devid, gen := range initVec {
		rgen, ok := respVec[devid]
		if ok {
			updateDevRange(devid, rgen, gen, rSt.diff)
		}
	}

	// Compute missing generations for devices not in initiator's vector but
	// in responder's vector.
	for devid, rgen := range respVec {
		if _, ok := initVec[devid]; !ok {
			updateDevRange(devid, rgen, 0, rSt.diff)
		}
	}
}

// makeWireLogRec creates a sync log record to send on the wire from a given
// local sync record.
func (rSt *responderState) makeWireLogRec(ctx *context.T, rec *LocalLogRec) (*interfaces.LogRec, error) {
	// Get the object value at the required version.
	key, version := rec.Metadata.ObjId, rec.Metadata.CurVers
	var rawValue *vom.RawBytes

	if rSt.sg {
		sg, err := getSGDataEntryByOID(ctx, rSt.st, key, version)
		if err != nil {
			return nil, err
		}
		rawValue, err = vom.RawBytesFromValue(sg)
		if err != nil {
			return nil, err
		}
	} else if !rec.Metadata.Delete && rec.Metadata.RecType == interfaces.NodeRec {
		var value []byte
		var err error
		value, err = watchable.GetAtVersion(ctx, rSt.st, []byte(key), nil, []byte(version))
		if err != nil {
			return nil, err
		}
		err = vom.Decode(value, &rawValue)
		if err != nil {
			return nil, err
		}
	}

	wireRec := &interfaces.LogRec{Metadata: rec.Metadata, Value: rawValue}
	return wireRec, nil
}

func updateDevRange(devid, rgen, gen uint64, gens genRangeVector) {
	if gen < rgen {
		// Need to include all generations in the interval [gen+1,rgen], gen+1 and rgen inclusive.
		if r, ok := gens[devid]; !ok {
			gens[devid] = &genRange{min: gen + 1, max: rgen}
		} else {
			if gen+1 < r.min {
				r.min = gen + 1
			}
			if rgen > r.max {
				r.max = rgen
			}
		}
	}
}

func extractAndSortPrefixes(vec interfaces.Knowledge) []string {
	pfxs := make([]string, len(vec))
	i := 0
	for p := range vec {
		pfxs[i] = p
		i++
	}
	sort.Strings(pfxs)
	return pfxs
}

// TODO(hpucha): This can be optimized using a scan instead of "gets" in a for
// loop.
func getNextLogRec(ctx *context.T, st store.Store, pfx string, dev uint64, r *genRange) (*LocalLogRec, error) {
	for i := r.cur; i <= r.max; i++ {
		rec, err := getLogRec(ctx, st, pfx, dev, i)
		if err == nil {
			r.cur = i + 1
			return rec, nil
		}
		if verror.ErrorID(err) != verror.ErrNoExist.ID {
			return nil, err
		}
	}
	return nil, nil
}

// Note: initPfxs is sorted.
func filterLogRec(rec *LocalLogRec, initVecs interfaces.Knowledge, initPfxs []string) bool {
	// The key (objid) starts with one of the store's reserved prefixes for
	// managed namespaces (e.g. "r" for row, "c" for collection perms). Remove
	// that prefix before comparing it with the syncgroup prefixes which are
	// defined by the application.
	key := common.StripFirstKeyPartOrDie(rec.Metadata.ObjId)

	filter := true
	var maxGen uint64
	for _, p := range initPfxs {
		if strings.HasPrefix(key, p) {
			// Do not filter. Initiator is interested in this
			// prefix.
			filter = false

			// Track if the initiator knows of this record.
			gen := initVecs[p][rec.Metadata.Id]
			if maxGen < gen {
				maxGen = gen
			}
		}
	}

	// Filter this record if the initiator already has it.
	if maxGen >= rec.Metadata.Gen {
		filter = true
	}

	return filter
}

// A minHeap implements heap.Interface and holds local log records.
type minHeap []*LocalLogRec

func (mh minHeap) Len() int { return len(mh) }

func (mh minHeap) Less(i, j int) bool {
	return mh[i].Pos < mh[j].Pos
}

func (mh minHeap) Swap(i, j int) {
	mh[i], mh[j] = mh[j], mh[i]
}

func (mh *minHeap) Push(x interface{}) {
	item := x.(*LocalLogRec)
	*mh = append(*mh, item)
}

func (mh *minHeap) Pop() interface{} {
	old := *mh
	n := len(old)
	item := old[n-1]
	*mh = old[0 : n-1]
	return item
}

// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/vom"
)

// DischargeRefreshFraction determines how early before their expiration
// time we refresh discharges.  A value of 0.5 means we refresh when it
// is only half way to is expiration time.
const DischargeRefreshFraction = 0.5

// If this is attached to the context, we will not fetch discharges.
// We use this to prevent ourselves from fetching discharges in the
// process of fetching discharges, thus creating an infinite loop.
type skipDischargesKey struct{}

type updateResult struct {
	refreshTime time.Time
	discharge   security.Discharge
	done        bool
}

type work struct {
	caveat  security.Caveat
	impetus security.DischargeImpetus
}

// PrepareDischarges retrieves the caveat discharges required for using blessings
// at server. The discharges are either found in the dischargeCache, in the call
// options, or requested from the discharge issuer indicated on the caveat.
// Note that requesting a discharge is an rpc call, so one copy of this
// function must be able to successfully terminate while another is blocked.
// PrepareDischarges also returns a refreshTime, which is the time at which
// PrepareDischarges should be called again (or zero if none of the discharges
// expire).
func PrepareDischarges(
	ctx *context.T,
	blessings security.Blessings,
	serverBlessings []string,
	method string,
	args []interface{}) (map[string]security.Discharge, time.Time) {
	tpCavs := blessings.ThirdPartyCaveats()
	if len(tpCavs) == 0 {
		return nil, time.Time{}
	}
	// We only want to send the impetus information we really need for each
	// discharge.
	todo := make(map[string]work, len(tpCavs))
	for _, cav := range tpCavs {
		if tp := cav.ThirdPartyDetails(); tp != nil {
			todo[tp.ID()] = work{cav, filteredImpetus(tp.Requirements(), serverBlessings, method, args)}
		}
	}
	// Since there may be dependencies in the caveats, we keep retrying
	// until either all discharges can be fetched or no new discharges
	// are fetched.
	var minRefreshTime time.Time
	ch := make(chan *updateResult, len(tpCavs))
	ret := make(map[string]security.Discharge, len(tpCavs))
	for {
		want := len(todo)
		now := time.Now()
		for _, w := range todo {
			updateDischarge(ctx, now, w.impetus, w.caveat, ch)
		}
		got := 0
		for i := 0; i < want; i++ {
			res := <-ch
			id := res.discharge.ID()
			ret[id] = res.discharge
			if res.done {
				minRefreshTime = minTime(minRefreshTime, res.refreshTime)
				delete(todo, id)
				got++
			}
		}
		if got == want {
			return ret, minRefreshTime
		}
		if got == 0 {
			return ret, minTime(minRefreshTime, time.Now().Add(time.Minute))
		}
	}
}

func updateDischarge(
	ctx *context.T,
	now time.Time,
	impetus security.DischargeImpetus,
	caveat security.Caveat,
	out chan<- *updateResult) {
	bstore := v23.GetPrincipal(ctx).BlessingStore()
	dis, ct := bstore.Discharge(caveat, impetus)
	if skip, _ := ctx.Value(skipDischargesKey{}).(bool); skip {
		// We can't fetch discharges while making a call to fetch a discharge.
		// Just go with what we have in the cache.
		out <- &updateResult{done: true, discharge: dis}
		return
	}
	if rt := refreshTime(dis, ct); dis.ID() != "" && (now.Before(rt) || rt.IsZero()) {
		// The cached value is still fresh, just return it.
		out <- &updateResult{done: true, discharge: dis, refreshTime: rt}
		return
	}
	// discharge in blessing store either doesn't exist or may be stale,
	// refresh via RPC.
	go func() {
		tp := caveat.ThirdPartyDetails()
		ctx = context.WithValue(ctx, skipDischargesKey{}, true)
		var newDis security.Discharge
		args, res := []interface{}{caveat, impetus}, []interface{}{&newDis}
		ctx.VI(3).Infof("Fetching discharge for %v", tp)
		if err := v23.GetClient(ctx).Call(ctx, tp.Location(), "Discharge", args, res); err != nil {
			ctx.VI(3).Infof("Discharge fetch for %v failed: %v", tp, err)
			out <- &updateResult{discharge: dis}
			return
		}
		bstore.CacheDischarge(newDis, caveat, impetus)
		out <- &updateResult{done: true, discharge: newDis, refreshTime: refreshTime(newDis, time.Now())}
	}()
}

// filteredImpetus returns a copy of 'before' after removing any values that are not required as per 'r'.
func filteredImpetus(r security.ThirdPartyRequirements, serverBlessings []string, method string, args []interface{}) security.DischargeImpetus {
	if !r.ReportServer {
		serverBlessings = nil
	}
	if !r.ReportMethod {
		method = ""
	}
	if !r.ReportArguments {
		args = nil
	}
	return mkDischargeImpetus(serverBlessings, method, args)
}

func refreshTime(dis security.Discharge, cacheTime time.Time) time.Time {
	expiry := dis.Expiry()
	if expiry.IsZero() {
		return time.Time{}
	}
	if cacheTime.IsZero() {
		// If we don't know the cache time, just try to refresh within a minute of
		// the expiry time.
		return expiry.Add(-time.Minute)
	}
	lifetime := expiry.Sub(cacheTime)
	return expiry.Add(-time.Duration(float64(lifetime) * DischargeRefreshFraction))
}

func minTime(a, b time.Time) time.Time {
	if a.IsZero() || (!b.IsZero() && b.Before(a)) {
		return b
	}
	return a
}

func mkDischargeImpetus(serverBlessings []string, method string, args []interface{}) security.DischargeImpetus {
	var impetus security.DischargeImpetus
	if len(serverBlessings) > 0 {
		impetus.Server = make([]security.BlessingPattern, len(serverBlessings))
		for i, b := range serverBlessings {
			impetus.Server[i] = security.BlessingPattern(b)
		}
	}
	impetus.Method = method
	if len(args) > 0 {
		impetus.Arguments = make([]*vom.RawBytes, len(args))
		for i, a := range args {
			vArg, err := vom.RawBytesFromValue(a)
			if err != nil {
				continue
			}
			impetus.Arguments[i] = vArg
		}
	}
	return impetus
}

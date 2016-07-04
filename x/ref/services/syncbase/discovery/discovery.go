// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package discovery

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/conventions"
	"v.io/v23/discovery"
	"v.io/v23/naming"
	"v.io/v23/security"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase/util"
	"v.io/x/lib/nsync"
	"v.io/x/ref/lib/discovery/global"
	"v.io/x/ref/services/syncbase/server/interfaces"
)

const (
	visibilityKey = "vis"
	appNameKey    = "appName"
	blessingsKey  = "blessings"

	nhDiscoveryKey = iota
	globalDiscoveryKey
)

// Discovery implements v.io/v23/discovery.T for syncbase based
// applications.
// TODO(mattr): Actually this is not syncbase specific.  At some
// point we should just replace the result of v23.NewDiscovery
// with this.
type Discovery struct {
	nhDiscovery     discovery.T
	globalDiscovery discovery.T
}

// NewDiscovery creates a new syncbase discovery object.
// globalDiscoveryPath is the path in the namespace where global disovery
// advertisements will be mounted.
// If globalDiscoveryPath is empty, no global discovery service will be created.
// globalScanInterval is the interval at which global discovery will be refreshed.
// If globalScanInterval is 0, the defaultScanInterval of global discovery will
// be used.
func NewDiscovery(ctx *context.T, globalDiscoveryPath string, globalScanInterval time.Duration) (discovery.T, error) {
	d := &Discovery{}
	var err error
	if d.nhDiscovery, err = v23.NewDiscovery(ctx); err != nil {
		return nil, err
	}
	if globalDiscoveryPath != "" {
		if d.globalDiscovery, err = global.NewWithTTL(ctx, globalDiscoveryPath, 0, globalScanInterval); err != nil {
			return nil, err
		}
	}
	return d, nil
}

// Scan implements v.io/v23/discovery/T.Scan.
func (d *Discovery) Scan(ctx *context.T, query string) (<-chan discovery.Update, error) {
	nhCtx, nhCancel := context.WithCancel(ctx)
	nhUpdates, err := d.nhDiscovery.Scan(nhCtx, query)
	if err != nil {
		nhCancel()
		return nil, err
	}
	var globalUpdates <-chan discovery.Update
	if d.globalDiscovery != nil {
		if globalUpdates, err = d.globalDiscovery.Scan(ctx, query); err != nil {
			nhCancel()
			return nil, err
		}
	}

	// Currently setting visibility on the neighborhood discovery
	// service turns IBE encryption on.  We currently don't have the
	// infrastructure support for IBE, so that would make our advertisements
	// unreadable by everyone.
	// Instead we add the visibility list to the attributes of the advertisement
	// and filter on the client side.  This is a temporary measure until
	// IBE is set up.  See v.io/i/1345.
	updates := make(chan discovery.Update)
	go func() {
		defer nhCancel()
		defer close(updates)
		seen := make(map[discovery.AdId]*updateRef)
		for {
			var u discovery.Update
			var src uint // key of the source discovery service where the update came from
			select {
			case <-ctx.Done():
				return
			case u = <-nhUpdates:
				src = nhDiscoveryKey
			case u = <-globalUpdates:
				src = globalDiscoveryKey
			}
			d.handleUpdate(ctx, u, src, seen, updates)
		}
	}()

	return updates, nil
}

func (d *Discovery) handleUpdate(ctx *context.T, u discovery.Update, src uint, seen map[discovery.AdId]*updateRef, updates chan discovery.Update) {
	if u == nil {
		return
	}
	patterns := splitPatterns(u.Attribute(visibilityKey))
	if len(patterns) > 0 && !matchesPatterns(ctx, patterns) {
		return
	}

	id := u.Id()
	prev := seen[id]
	if u.IsLost() {
		// Only send the lost noitification if a found event was previously seen,
		// and all discovery services that found it have lost it.
		if prev == nil || !prev.unset(src) {
			return
		}
		delete(seen, id)
		updates <- update{Update: u, lost: true}
		return
	}

	if prev == nil {
		// Always send updates for updates that we have never seen before.
		ref := &updateRef{update: u}
		ref.set(src)
		seen[id] = ref
		updates <- update{Update: u}
		return
	}

	if differ := updatesDiffer(prev.update, u); (differ && u.Timestamp().After(prev.update.Timestamp())) ||
		(!differ && src == nhDiscoveryKey && len(u.Advertisement().Attachments) > 0) {
		// If the updates differ and the newly found update has a later time than
		// previously found one, lose prev and find new.
		// Or, if the update doesn't differ, but is from neighborhood discovery, it
		// could have more information since we don't yet encode attachements in
		// global discovery.
		updates <- update{Update: prev.update, lost: true}
		ref := &updateRef{update: u}
		ref.set(src)
		seen[id] = ref
		updates <- update{Update: u}
		return
	}
}

// Advertise implements v.io/v23/discovery/T.Advertise.
func (d *Discovery) Advertise(ctx *context.T, ad *discovery.Advertisement, visibility []security.BlessingPattern) (<-chan struct{}, error) {
	// Currently setting visibility on the neighborhood discovery
	// service turns IBE encryption on.  We currently don't have the
	// infrastructure support for IBE, so that would make our advertisements
	// unreadable by everyone.
	// Instead we add the visibility list to the attributes of the advertisement
	// and filter on the client side.  This is a temporary measure until
	// IBE is set up.  See v.io/i/1345.
	adCopy := *ad
	if len(visibility) > 0 {
		adCopy.Attributes = make(discovery.Attributes, len(ad.Attributes)+1)
		for k, v := range ad.Attributes {
			adCopy.Attributes[k] = v
		}
		patterns := joinPatterns(visibility)
		adCopy.Attributes[visibilityKey] = patterns
	}

	stopped := make(chan struct{})
	nhCtx, nhCancel := context.WithCancel(ctx)
	nhStopped, err := d.nhDiscovery.Advertise(nhCtx, &adCopy, nil)
	if err != nil {
		nhCancel()
		return nil, err
	}
	var globalStopped <-chan struct{}
	if d.globalDiscovery != nil {
		if globalStopped, err = d.globalDiscovery.Advertise(ctx, &adCopy, nil); err != nil {
			nhCancel()
			<-nhStopped
			return nil, err
		}
	}
	go func() {
		<-nhStopped
		if d.globalDiscovery != nil {
			<-globalStopped
		}
		nhCancel()
		close(stopped)
	}()
	ad.Id = adCopy.Id
	return stopped, nil
}

func updatesDiffer(a, b discovery.Update) bool {
	if !sortedStringsEqual(a.Addresses(), b.Addresses()) {
		return true
	}
	if !mapsEqual(a.Advertisement().Attributes, b.Advertisement().Attributes) {
		return true
	}
	return false
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for ka, va := range a {
		if vb, ok := b[ka]; !ok || va != vb {
			return false
		}
	}
	return true
}

func sortedStringsEqual(a, b []string) bool {
	// We want to make a nil and an empty slices equal to avoid unnecessary inequality by that.
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func matchesPatterns(ctx *context.T, patterns []security.BlessingPattern) bool {
	p := v23.GetPrincipal(ctx)
	blessings := p.BlessingStore().PeerBlessings()
	for _, b := range blessings {
		names := security.BlessingNames(p, b)
		for _, pattern := range patterns {
			if pattern.MatchedBy(names...) {
				return true
			}
		}
	}
	return false
}

type updateRef struct {
	update    discovery.Update
	nhRef     bool
	globalRef bool
}

func (r *updateRef) set(d uint) {
	switch d {
	case nhDiscoveryKey:
		r.nhRef = true
	case globalDiscoveryKey:
		r.globalRef = true
	}
}

func (r *updateRef) unset(d uint) bool {
	switch d {
	case nhDiscoveryKey:
		r.nhRef = false
	case globalDiscoveryKey:
		r.globalRef = false
	}
	return !r.nhRef && !r.globalRef
}

// update wraps the discovery.Update to remove the visibility attribute which we add
// and allows us to mark the update as lost.
type update struct {
	discovery.Update
	lost bool
}

func (u update) IsLost() bool { return u.lost }

func (u update) Attribute(name string) string {
	if name == visibilityKey {
		return ""
	}
	return u.Update.Attribute(name)
}

func (u update) Advertisement() discovery.Advertisement {
	cp := u.Update.Advertisement()
	orig := cp.Attributes
	cp.Attributes = make(discovery.Attributes, len(orig))
	for k, v := range orig {
		if k != visibilityKey {
			cp.Attributes[k] = v
		}
	}
	return cp
}

// blessingSeparator is used to join multiple blessings into a
// single string.
// Note that comma cannot appear in blessings, see:
// v.io/v23/security/certificate.go
const blessingsSeparator = ','

// joinPatterns concatenates the elements of a to create a single string.
// The string can be split again with SplitPatterns.
func joinPatterns(a []security.BlessingPattern) string {
	if len(a) == 0 {
		return ""
	}
	if len(a) == 1 {
		return string(a[0])
	}
	n := (len(a) - 1)
	for i := 0; i < len(a); i++ {
		n += len(a[i])
	}

	b := make([]byte, n)
	bp := copy(b, a[0])
	for _, s := range a[1:] {
		b[bp] = blessingsSeparator
		bp++
		bp += copy(b[bp:], s)
	}
	return string(b)
}

// splitPatterns splits BlessingPatterns that were joined with
// JoinBlessingPattern.
func splitPatterns(patterns string) []security.BlessingPattern {
	if patterns == "" {
		return nil
	}
	n := strings.Count(patterns, string(blessingsSeparator)) + 1
	out := make([]security.BlessingPattern, n)
	last, start := 0, 0
	for i, r := range patterns {
		if r == blessingsSeparator {
			out[last] = security.BlessingPattern(patterns[start:i])
			last++
			start = i + 1
		}
	}
	out[last] = security.BlessingPattern(patterns[start:])
	return out
}

var state struct {
	mu    nsync.Mu
	scans map[security.Principal]*scanState
}

type scanState struct {
	peers     *copyableQueue
	appPeers  map[string]*copyableQueue
	dbs       map[wire.Id]*copyableQueue
	listeners int
	cancel    context.CancelFunc
}

func newScan(ctx *context.T) (*scanState, error) {
	ctx, cancel := context.WithRootCancel(ctx)
	scan := &scanState{
		peers:    nil,
		appPeers: make(map[string]*copyableQueue),
		dbs:      make(map[wire.Id]*copyableQueue),
		cancel:   cancel,
	}
	// TODO(suharshs): Add globalDiscoveryPath.
	d, err := NewDiscovery(ctx, "", 0)
	if err != nil {
		scan.cancel()
		return nil, err
	}
	query := fmt.Sprintf("v.InterfaceName=\"%s/%s\"",
		interfaces.SyncDesc.PkgPath, interfaces.SyncDesc.Name)
	updates, err := d.Scan(ctx, query)
	if err != nil {
		scan.cancel()
		return nil, err
	}
	go func() {
		for u := range updates {
			if invite, db, ok := makeInvite(u.Advertisement()); ok {
				state.mu.Lock()
				q := scan.dbs[db]
				if u.IsLost() {
					// TODO(mattr): Removing like this can result in resurfacing already
					// retrieved invites.  For example if the ACL of a syncgroup is
					// changed to add a new member, then we might see a remove and then
					// later an add.  In this case we would end up moving the invite
					// to the end of the queue.  One way to fix this would be to
					// keep removed invites for some time.
					if q != nil {
						q.remove(invite)
						if q.empty() {
							delete(scan.dbs, db)
						}
					}
				} else {
					if q == nil {
						q = newCopyableQueue()
						scan.dbs[db] = q
					}
					q.add(invite)
					q.cond.Broadcast()
				}
				state.mu.Unlock()
			} else if peer, ok := makePeer(u.Advertisement()); ok {
				state.mu.Lock()
				q := scan.peers
				if u.IsLost() {
					if q != nil {
						q.remove(peer)
						if q.empty() {
							scan.peers = nil
						}
					}
				} else {
					if q == nil {
						q = newCopyableQueue()
						scan.peers = q
					}
					q.add(peer)
					q.cond.Broadcast()
				}
				state.mu.Unlock()
			} else if peer, ok := makeAppPeer(u.Advertisement()); ok {
				state.mu.Lock()
				app := peer.AppName
				q := scan.appPeers[app]
				if u.IsLost() {
					if q != nil {
						q.remove(peer)
						if q.empty() {
							delete(scan.appPeers, app)
							scan.peers = nil
						}
					}
				} else {
					if q == nil {
						q = newCopyableQueue()
						scan.appPeers[app] = q
					}
					q.add(peer)
					q.cond.Broadcast()
				}
				state.mu.Unlock()
			}
		}
	}()
	return scan, nil
}

// ListenForInvites listens via Discovery for syncgroup invitations for the given
// database and sends the invites to the provided channel.  We stop listening when
// the given context is canceled.  When that happens we close the given channel.
func ListenForInvites(ctx *context.T, db wire.Id, ch chan<- Invite) error {
	defer state.mu.Unlock()
	state.mu.Lock()

	scan, err := prepareScannerByPrincipal(ctx)
	if err != nil {
		return err
	}

	q := scan.dbs[db]
	if q == nil {
		q = newCopyableQueue()
		scan.dbs[db] = q
	}

	// Send the copyables into a temporary channel.
	cp_ch := make(chan copyable)
	go consumeCopyableQueue(ctx, q, scan, cp_ch, func() {
		delete(scan.dbs, db)
	})

	// Convert these copyables into Invites.
	go func() {
		for {
			v, ok := <-cp_ch
			if !ok {
				close(ch)
				break
			}
			invite := v.(Invite)
			if !invite.isLost() {
				ch <- invite
			}
		}
	}()

	return nil
}

// ListenForPeers listens via Discovery for syncgroup peers and sends them to
// the provided channel.  We stop listening when the context is canceled.  When
// that happens we close the given channel.
func ListenForPeers(ctx *context.T, ch chan<- Peer) error {
	defer state.mu.Unlock()
	state.mu.Lock()

	scan, err := prepareScannerByPrincipal(ctx)
	if err != nil {
		return err
	}

	q := scan.peers
	if q == nil {
		q = newCopyableQueue()
		scan.peers = q
	}

	// Send the copyables into a temporary channel.
	cp_ch := make(chan copyable)
	go consumeCopyableQueue(ctx, q, scan, cp_ch, func() {
		scan.peers = nil
	})

	// Convert these copyables into Peers.
	go func() {
		for {
			v, ok := <-cp_ch
			if !ok {
				close(ch)
				break
			}
			ch <- v.(Peer)
		}
	}()

	return nil
}

// AdvertiseApp advertises that this peer is running their app with syncbase.
func AdvertiseApp(ctx *context.T, visibility []security.BlessingPattern) (done <-chan struct{}, err error) {
	d, err := NewDiscovery(ctx, "", 0)
	if err != nil {
		return nil, err
	}

	user, app, err := extractUserAndAppFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Create an advertisement.
	// Note: The interface name doesn't have to be syncbase's. If we change it, we
	// should update the Scan query above as well.
	interfaceName := fmt.Sprintf("%s/%s", interfaces.SyncDesc.PkgPath, interfaces.SyncDesc.Name)
	ad := &discovery.Advertisement{
		InterfaceName: interfaceName,
		Addresses:     []string{""}, // Addresses are required, but we don't have any.
		Attributes:    discovery.Attributes{appNameKey: app, blessingsKey: user},
	}

	done, err = d.Advertise(ctx, ad, visibility)
	return
}

func extractUserAndAppFromContext(ctx *context.T) (string, string, error) {
	// Use a utility method to cut up the context's blessings and extract just
	// the app-user blessing.
	_, userPattern, err := util.AppAndUserPatternFromBlessings(security.DefaultBlessingNames(v23.GetPrincipal(ctx))...)
	if err != nil {
		return "", "", err
	}

	// Obtain the app component only.
	app := conventions.ParseBlessingNames(string(userPattern))[0].Application
	if app == "" {
		return "", "", fmt.Errorf("context's user blessing did not specify an application")
	}
	return string(userPattern), app, nil
}

// ListenForAppPeers listens via Discovery for peers that are running the
// same application as their context's blessing. Updates are sent through the
// provided channel. We stop listening and close the channel when the context
// is canceled.
func ListenForAppPeers(ctx *context.T, ch chan<- AppPeer) error {
	defer state.mu.Unlock()
	state.mu.Lock()

	_, app, err := extractUserAndAppFromContext(ctx)
	if err != nil {
		return err
	}

	scan, err := prepareScannerByPrincipal(ctx)
	if err != nil {
		return err
	}

	q := scan.appPeers[app]
	if q == nil {
		q = newCopyableQueue()
		scan.appPeers[app] = q
	}

	// Send the copyables into a temporary channel.
	cp_ch := make(chan copyable)
	go consumeCopyableQueue(ctx, q, scan, cp_ch, func() {
		scan.appPeers[app] = nil
	})

	// Convert these copyables into AppPeers.
	go func() {
		for {
			v, ok := <-cp_ch
			if !ok {
				close(ch)
				break
			}
			ch <- v.(AppPeer)
		}
	}()

	return nil
}

func prepareScannerByPrincipal(ctx *context.T) (*scanState, error) {
	p := v23.GetPrincipal(ctx)
	scan := state.scans[p]
	if scan == nil {
		var err error
		if scan, err = newScan(ctx); err != nil {
			return nil, err
		}
		if state.scans == nil {
			state.scans = make(map[security.Principal]*scanState)
		}
		state.scans[p] = scan
	}
	scan.listeners++
	return scan, nil
}

type isQueueEmptyCallback func()

// Should be called in a goroutine. Pairs with prepareScannerByPrincipal.
func consumeCopyableQueue(ctx *context.T, q *copyableQueue, scan *scanState, ch chan<- copyable, cb isQueueEmptyCallback) {
	c := q.scan()
	for {
		next, ok := q.next(ctx, c)
		if !ok {
			break
		}
		select {
		case ch <- next.copy():
		case <-ctx.Done():
			q.stopScan(c)
			break
		}
	}
	close(ch)

	defer state.mu.Unlock()
	state.mu.Lock()

	if q.empty() {
		cb()
	}
	scan.listeners--
	if scan.listeners == 0 {
		scan.cancel()
		delete(state.scans, v23.GetPrincipal(ctx))
	}
}

// copyableQueue is a linked list based queue. As we get new invitations/peers,
// we add them to the queue, and when advertisements are lost we remove instead.
// Each call to scan creates a cursor on the queue that will iterate until
// the Listen is canceled.  We wait when we hit the end of the queue via the
// condition variable cond.
// Note: The queue contains both "found" and "lost" elements. A removed "found"
// element is replaced with a "lost" element, which all cursors will see.
// See ielement for the garbage collection strategy for the "lost" elements.
type copyableQueue struct {
	mu   nsync.Mu
	cond nsync.CV

	elems        map[string]*ielement
	sentinel     ielement
	cursors      map[int]*ielement
	nextCursorId int
}

func (q *copyableQueue) debugLocked() string {
	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, "*%p", &q.sentinel)
	for c := q.sentinel.next; c != &q.sentinel; c = c.next {
		fmt.Fprintf(buf, " %p", c)
	}
	return buf.String()
}

// An interface used to represent Invite and Peer that reduces code duplication.
// Typecasting is cheap, so this seems better than maintaining duplicate
// implementations of copyableQueue.
type copyable interface {
	copy() copyable
	id() string
	isLost() bool
	copyAsLost() copyable
}

// Keeps track of a copyable element and the next/prev ielements in a queue.
// ielements can represent both "found" and "lost" elements. Those of the "lost"
// type also set "numCanSee" and "whoCanSee" to indicate that only certain
// cursors can see the element contained in this ielement. Once all the cursors
// have seen a "lost" ielement it can be garbage collected safely.
// See "canBeSeenByCursor" and "consume".
type ielement struct {
	elem       copyable
	prev, next *ielement
	numCanSee  int   // If >0, this decrements. The element is removed upon hitting 0.
	whoCanSee  []int // The cursors that can see this ielement. Set only once.
}

// A cursor can see the element in this ielement if it is a "found" element or
// a "lost" element with the correct visibility.
func (i *ielement) canBeSeenByCursor(cursor int) bool {
	if i.numCanSee == 0 { // Special: 0 is visible by everyone.
		return true
	}
	for _, who := range i.whoCanSee {
		if cursor == who {
			return true
		}
	}
	return false
}

// Reduce "numCanSee" and indicate whether or not this element needs to be removed
// from the queue.
func (i *ielement) consume() bool {
	if i.numCanSee > 0 {
		i.numCanSee--
		return i.numCanSee == 0
	}
	return false
}

func newCopyableQueue() *copyableQueue {
	iq := &copyableQueue{
		elems:   make(map[string]*ielement),
		cursors: make(map[int]*ielement),
	}
	iq.sentinel.next, iq.sentinel.prev = &iq.sentinel, &iq.sentinel
	return iq
}

func (q *copyableQueue) empty() bool {
	defer q.mu.Unlock()
	q.mu.Lock()
	return len(q.elems) == 0 && len(q.cursors) == 0
}

func (q *copyableQueue) size() int {
	defer q.mu.Unlock()
	q.mu.Lock()
	return len(q.elems)
}

// Removes the given copyable from the queue. No-op if not present or "lost".
// Note: As described in copyableQueue, if the copyable is a "found" element,
// then a corresponding "lost" element will be appended to the queue. Only the
// cursors who have seen the "found" element can see the "lost" one.
func (q *copyableQueue) remove(i copyable) {
	defer q.mu.Unlock()
	q.mu.Lock()

	el, ok := q.elems[i.id()]
	if !ok || el.elem.isLost() {
		return
	}

	// Make a list of all the users who have seen this element.
	whoCanSee := []int{}
	for cursor, cel := range q.cursors {
		// Adjust the cursor if it is pointing to the removed element.
		if cel == el {
			q.cursors[cursor] = cel.prev
			whoCanSee = append(whoCanSee, cursor)
			continue
		}

		// Check if this cursor has gone past the removed element.
		// Note: This implementation assumes a small queue and a small number of
		// cursors. Otherwise, using a table is recommended over list traversal.
		for ; cel != &q.sentinel; cel = cel.prev {
			if cel == el {
				whoCanSee = append(whoCanSee, cursor)
				break
			}
		}
	}

	// "Remove" el from the queue.
	el.next.prev, el.prev.next = el.prev, el.next

	// el is a visible lost item that needs to be added to the queue.
	if len(whoCanSee) > 0 {
		// Adjust the lost ielement el to include the lost data and who can see it.
		el.elem = el.elem.copyAsLost()
		el.numCanSee = len(whoCanSee)
		el.whoCanSee = whoCanSee

		// Insert the lost element before/after the sentinel and broadcast.
		el.prev, el.next = q.sentinel.prev, &q.sentinel
		q.sentinel.prev, q.sentinel.prev.next = el, el
		q.cond.Broadcast()
	} else {
		delete(q.elems, i.id())
	}
}

func (q *copyableQueue) add(i copyable) {
	defer q.mu.Unlock()
	q.mu.Lock()

	if _, ok := q.elems[i.id()]; ok {
		return
	}
	el := &ielement{i, q.sentinel.prev, &q.sentinel, 0, []int{}}
	q.sentinel.prev, q.sentinel.prev.next = el, el
	q.elems[i.id()] = el
	q.cond.Broadcast()
}

func (q *copyableQueue) next(ctx *context.T, cursor int) (copyable, bool) {
	defer q.mu.Unlock()
	q.mu.Lock()
	c, exists := q.cursors[cursor]
	if !exists {
		return nil, false
	}

	// Find the next available element that this cursor can see. Will block until
	// somebody writes to the queue or the given context is canceled.
	// Note: Some "lost" elements in the queue will not be visible to this cursor;
	// these are skipped.
	for {
		for c.next == &q.sentinel {
			if q.cond.WaitWithDeadline(&q.mu, nsync.NoDeadline, ctx.Done()) != nsync.OK {
				q.removeCursorLocked(cursor)
				return nil, false
			}
			c = q.cursors[cursor]
		}
		c = c.next
		q.cursors[cursor] = c
		if c.canBeSeenByCursor(cursor) {
			if c.consume() {
				q.removeLostIElemLocked(c)

			}
			return c.elem, true
		}
	}
}

func (q *copyableQueue) removeCursorLocked(cursor int) {
	c := q.cursors[cursor]
	delete(q.cursors, cursor)

	// Garbage collection: Step through each remaining element and remove
	// unneeded lost ielements.
	for c.next != &q.sentinel {
		c = c.next // step
		if c.canBeSeenByCursor(cursor) {
			if c.consume() {
				q.removeLostIElemLocked(c)
			}
		}
	}
}

func (q *copyableQueue) removeLostIElemLocked(ielem *ielement) {
	// Remove the lost ielem since everyone has seen it.
	delete(q.elems, ielem.elem.id())
	ielem.next.prev, ielem.prev.next = ielem.prev, ielem.next

	// Adjust the cursors if their elements were on the removed element.
	for id, c := range q.cursors {
		if c == ielem {
			q.cursors[id] = c.prev
		}
	}
}

func (q *copyableQueue) scan() int {
	defer q.mu.Unlock()
	q.mu.Lock()
	id := q.nextCursorId
	q.nextCursorId++
	q.cursors[id] = &q.sentinel
	return id
}

func (q *copyableQueue) stopScan(cursor int) {
	defer q.mu.Unlock()
	q.mu.Lock()

	if _, exists := q.cursors[cursor]; exists {
		q.removeCursorLocked(cursor)
	}
}

// Invite represents an invitation to join a syncgroup as found via Discovery.
type Invite struct {
	Syncgroup     wire.Id  // Syncgroup is the Id of the syncgroup you've been invited to.
	Addresses     []string // Addresses are the list of addresses of the inviting server.
	BlessingNames []string // BlessingNames are the list of blessings of the inviting server.
	Lost          bool     // If this invite is a lost invite or not.
	key           string   // Unexported. The implementation uses this key to de-dupe ads.
}

func makeInvite(ad discovery.Advertisement) (Invite, wire.Id, bool) {
	var dbId, sgId wire.Id
	var ok bool
	if dbId.Name, ok = ad.Attributes[wire.DiscoveryAttrDatabaseName]; !ok {
		return Invite{}, dbId, false
	}
	if dbId.Blessing, ok = ad.Attributes[wire.DiscoveryAttrDatabaseBlessing]; !ok {
		return Invite{}, dbId, false
	}
	if sgId.Name, ok = ad.Attributes[wire.DiscoveryAttrSyncgroupName]; !ok {
		return Invite{}, dbId, false
	}
	if sgId.Blessing, ok = ad.Attributes[wire.DiscoveryAttrSyncgroupBlessing]; !ok {
		return Invite{}, dbId, false
	}
	i := Invite{
		Addresses:     append([]string{}, ad.Addresses...),
		BlessingNames: computeBlessingNamesFromEndpoints(ad.Addresses),
		Syncgroup:     sgId,
	}
	sort.Strings(i.Addresses)
	i.key = fmt.Sprintf("%v", i)
	return i, dbId, true
}

// computeBlessingNamesFromEndpoints parses endpoints and concatenates found blessings together.
func computeBlessingNamesFromEndpoints(addresses []string) []string {
	blessingsMap := make(map[string]struct{})

	for _, address := range addresses {
		if e, err := naming.ParseEndpoint(address); err != nil {
			for _, name := range e.BlessingNames() {
				blessingsMap[name] = struct{}{}
			}
		}
	}

	blessingNames := make([]string, len(blessingsMap))
	for blessings, _ := range blessingsMap {
		blessingNames = append(blessingNames, blessings)
	}
	return blessingNames
}

func (i Invite) copy() copyable {
	cp := i
	cp.Addresses = append([]string(nil), i.Addresses...)
	cp.BlessingNames = append([]string(nil), i.BlessingNames...)
	return cp
}

func (i Invite) id() string {
	return i.key
}

func (i Invite) isLost() bool {
	return i.Lost
}

func (i Invite) copyAsLost() copyable {
	cp := i
	cp.Addresses = append([]string(nil), i.Addresses...)
	cp.BlessingNames = append([]string(nil), i.BlessingNames...)
	cp.Lost = true
	return cp
}

// Peer represents a Syncbase peer found via Discovery.
type Peer struct {
	Name      string   // Name is the name of the Syncbase peer's sync service.
	Addresses []string // Addresses are the list of addresses of the peer's server.
	Lost      bool     // If this peer is a lost peer or not.
	key       string   // Unexported. The implementation uses this key to de-dupe ads.
}

// Attempts to make a Peer using its discovery attributes.
// Will return an empty Peer struct if it fails.
func makePeer(ad discovery.Advertisement) (Peer, bool) {
	peerName, ok := ad.Attributes[wire.DiscoveryAttrPeer]
	if !ok {
		return Peer{}, false
	}
	p := Peer{
		Name:      peerName,
		Addresses: append([]string{}, ad.Addresses...),
	}
	sort.Strings(p.Addresses)
	p.key = fmt.Sprintf("%v", p)
	return p, true
}

func (p Peer) copy() copyable {
	cp := p
	cp.Addresses = append([]string(nil), p.Addresses...)
	return cp
}

func (p Peer) id() string {
	return p.key
}

func (p Peer) isLost() bool {
	return p.Lost
}

func (p Peer) copyAsLost() copyable {
	cp := p
	cp.Addresses = append([]string(nil), p.Addresses...)
	cp.Lost = true
	return cp
}

// App Peer represents a Syncbase app peer found via Discovery.
// TODO(alexfandrianto): Can we include more than this? Peer Name? Addresses?
type AppPeer struct {
	AppName   string // The name of the app.
	Blessings string // The blessings pattern for this app peer.
	Lost      bool   // If this peer is a lost peer or not.
	key       string // Unexported. The implementation uses this key to de-dupe ads.
}

// Attempts to make an AppPeer using its discovery attributes.
// Will return an empty AppPeer struct if it fails.
func makeAppPeer(ad discovery.Advertisement) (AppPeer, bool) {
	appName, ok := ad.Attributes[appNameKey]
	if !ok {
		return AppPeer{}, false
	}
	blessings, ok := ad.Attributes[blessingsKey]
	if !ok {
		return AppPeer{}, false
	}
	p := AppPeer{
		AppName:   appName,
		Blessings: blessings,
	}
	p.key = fmt.Sprintf("%v", p)
	return p, true
}

func (p AppPeer) copy() copyable {
	cp := p
	return cp
}

func (p AppPeer) id() string {
	return p.key
}

func (p AppPeer) isLost() bool {
	return p.Lost
}

func (p AppPeer) copyAsLost() copyable {
	cp := p
	cp.Lost = true
	return cp
}

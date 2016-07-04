// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

// Package vsync provides sync functionality for Syncbase. Sync
// service serves incoming GetDeltas requests and contacts other peers
// to get deltas from them. When it receives a GetDeltas request, the
// incoming generation vector is diffed with the local generation
// vector, and missing generations are sent back. When it receives log
// records in response to a GetDeltas request, it replays those log
// records to get in sync with the sender.
import (
	"container/list"
	"fmt"
	"math/rand"
	"path"
	"sync"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/discovery"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/verror"
	"v.io/x/lib/vlog"
	idiscovery "v.io/x/ref/lib/discovery"
	"v.io/x/ref/lib/stats"
	"v.io/x/ref/services/syncbase/common"
	syncdis "v.io/x/ref/services/syncbase/discovery"
	blob "v.io/x/ref/services/syncbase/localblobstore"
	fsblob "v.io/x/ref/services/syncbase/localblobstore/fs_cablobstore"
	"v.io/x/ref/services/syncbase/server/interfaces"
	"v.io/x/ref/services/syncbase/store"
	"v.io/x/ref/services/syncbase/vclock"
)

// syncService contains the metadata for the sync module.
type syncService struct {
	// TODO(hpucha): see if "v.io/v23/uniqueid" is a better fit. It is 128
	// bits. Another alternative is to derive this from the blessing of
	// Syncbase. Syncbase can append a uuid to the blessing it is given upon
	// launch and use its hash as id. Note we cannot use public key since we
	// want to support key rollover.
	id   uint64 // globally unique id for this instance of Syncbase.
	name string // name derived from the global id.
	sv   interfaces.Service

	// Root context. Used for example to create a context for advertising
	// over neighborhood.
	ctx *context.T

	// Random number generator for this Sync service.
	rng     *rand.Rand
	rngLock sync.Mutex

	// State to coordinate shutdown of spawned goroutines.
	pending sync.WaitGroup
	closed  chan struct{}

	// TODO(hpucha): Other global names to advertise to enable Syncbase
	// discovery. For example, every Syncbase must be reachable under
	// <mttable>/<syncbaseid> for p2p sync. This is the name advertised
	// during syncgroup join. In addition, a Syncbase might also be
	// accepting "publish syncgroup requests", and might use a more
	// human-readable name such as <mttable>/<idp>/<sgserver>. All these
	// names must be advertised in the appropriate mount tables.

	// In-memory sync membership info aggregated across databases.
	allMembers     *memberView
	allMembersLock sync.RWMutex

	// In-memory maps of neighborhood information found via the discovery
	// service.  The IDs map gathers all the neighborhood info using the
	// instance IDs as keys.  The sync peers map is a secondary index to
	// access the info using the Syncbase names as keys.  The syncgroups
	// map is a secondary index to access the info using the syncgroup Gids
	// as keys of the outer-map, and instance IDs as keys of the inner-map.
	discovery           discovery.T
	discoveryIds        map[string]*discovery.Advertisement
	discoveryPeers      map[string]*discovery.Advertisement
	discoverySyncgroups map[interfaces.GroupId]map[string]*discovery.Advertisement
	discoveryLock       sync.RWMutex

	nameLock sync.Mutex // lock needed to serialize adding and removing of Syncbase names.

	// Handle to the server for adding other names in the future.
	svr rpc.Server

	// Cancel functions for contexts derived from the root context when
	// advertising over neighborhood. This is needed to stop advertising.
	cancelAdvSyncbase context.CancelFunc                            // cancels Syncbase advertising.
	advSyncgroups     map[interfaces.GroupId]syncAdvertisementState // describes syncgroup advertising.
	advLock           sync.Mutex

	// Whether to enable neighborhood advertising.
	publishInNh bool

	// In-memory sync state per Database. This state is populated at
	// startup, and periodically persisted by the initiator.
	syncState     map[wire.Id]*dbSyncStateInMem
	syncStateLock sync.Mutex // lock to protect access to the sync state.

	// In-memory queue of syncgroups to be published.  It is reconstructed
	// at startup from syncgroup info so it does not need to be persisted.
	sgPublishQueue     *list.List
	sgPublishQueueLock sync.Mutex

	// In-memory tracking of batches during their construction.
	// The sync Initiator and Watcher build batches incrementally here
	// and then persist them in DAG batch entries.  The mutex guards
	// access to the batch set.
	batchesLock sync.Mutex
	batches     batchSet

	// Metadata related to blob handling.
	bst blob.BlobStore // local blob store associated with this Syncbase.

	// Syncbase vclock related variables.
	vclock *vclock.VClock

	// Peer manager for managing peers to sync with.
	pm peerManager

	// Naming prefix at which debugging information is exported.
	statPrefix string
}

// syncDatabase contains the metadata for syncing a database. This struct is
// used as a receiver to hand off the app-initiated syncgroup calls that arrive
// against a database to the sync module.
type syncDatabase struct {
	db   interfaces.Database
	sync interfaces.SyncServerMethods
}

// syncAdvertisementState contains information about the most recent
// advertisement for each advertised syncgroup.
type syncAdvertisementState struct {
	cancel      context.CancelFunc // cancels advertising.
	specVersion string             // version of most recently advertised spec.
	adId        discovery.AdId
}

var (
	ifName                              = interfaces.SyncDesc.PkgPath + "/" + interfaces.SyncDesc.Name
	_      interfaces.SyncServerMethods = (*syncService)(nil)
)

// New creates a new sync module.
//
// Concurrency: sync initializes 3 goroutines at startup: a "syncer", a
// "neighborhood scanner", and a "peer manager".  In addition, one "watcher"
// thread is started for each database to watch its store for changes to its
// objects. The "syncer" thread is responsible for periodically contacting peers
// to fetch changes from them. The "neighborhood scanner" thread continuously
// scans the neighborhood to learn of other Syncbases and syncgroups in its
// neighborhood. The "peer manager" thread continuously maintains viable peers
// that the syncer can pick from. In addition, the sync module responds to
// incoming RPCs from remote sync modules and local clients.
func New(ctx *context.T, sv interfaces.Service, blobStEngine, blobRootDir string, cl *vclock.VClock, publishInNh bool) (*syncService, error) {
	// TODO(suharshs): Enable global discovery.
	discovery, err := syncdis.NewDiscovery(v23.WithListenSpec(ctx, rpc.ListenSpec{}), "", 0)
	if err != nil {
		return nil, err
	}
	s := &syncService{
		sv:             sv,
		batches:        make(batchSet),
		sgPublishQueue: list.New(),
		vclock:         cl,
		ctx:            ctx,
		rng:            rand.New(rand.NewSource(time.Now().UTC().UnixNano())),
		discovery:      discovery,
		publishInNh:    publishInNh,
		advSyncgroups:  make(map[interfaces.GroupId]syncAdvertisementState),
		statPrefix:     syncServiceStatName(),
	}
	s.exportStats()

	data := &SyncData{}
	if err := store.RunInTransaction(sv.St(), func(tx store.Transaction) error {
		if err := store.Get(ctx, sv.St(), s.stKey(), data); err != nil {
			if verror.ErrorID(err) != verror.ErrNoExist.ID {
				return err
			}
			// First invocation of vsync.New().
			// TODO(sadovsky): Maybe move guid generation and storage to serviceData.
			data.Id = s.rand64()
			return store.Put(ctx, tx, s.stKey(), data)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// data.Id is now guaranteed to be initialized.
	s.id = data.Id
	s.name = syncbaseIdToName(s.id)

	vlog.VI(1).Infof("sync: New: Syncbase ID is %x", s.id)

	// Initialize in-memory state for the sync module before starting any threads.
	if err := s.initSync(ctx); err != nil {
		return nil, verror.New(verror.ErrInternal, ctx, err)
	}

	// Open a blob store.
	s.bst, err = fsblob.Create(ctx, blobStEngine, path.Join(blobRootDir, "blobs"))
	if err != nil {
		return nil, err
	}

	// Channel to propagate close event to all threads.
	s.closed = make(chan struct{})
	s.pending.Add(3)

	// Initialize a peer manager with the peer selection policy.
	s.pm = newPeerManager(ctx, s, selectRandom)

	// Start the peer manager thread to maintain peers viable for syncing.
	go s.pm.managePeers(ctx)
	s.pm.exportStats(naming.Join("syncbase", s.name))

	// Start initiator thread to periodically get deltas from peers. The
	// initiator threads consults the peer manager to pick peers to sync
	// with. So we initialize the peer manager before starting the syncer.
	go s.syncer(ctx)

	// Start the discovery service thread to listen to neighborhood updates.
	go s.discoverNeighborhood(ctx)

	return s, nil
}

// Close waits for spawned sync threads to shut down, and closes the local blob
// store handle.
func Close(ss interfaces.SyncServerMethods) {
	vlog.VI(2).Infof("sync: Close: begin")
	defer vlog.VI(2).Infof("sync: Close: end")

	s := ss.(*syncService)
	s.stopAdvertisingSyncbaseInNeighborhood()
	close(s.closed)
	s.pending.Wait()
	s.bst.Close()
	stats.Delete(s.statPrefix)
}

func NewSyncDatabase(db interfaces.Database) *syncDatabase {
	return &syncDatabase{db: db, sync: db.Service().Sync()}
}

//////////////////////////////////////////////////////////////////////////////////////////
// Neighborhood based discovery of syncgroups and Syncbases.

// discoverNeighborhood listens to updates from the discovery service to learn
// about sync peers and syncgroups (if they have admins in the neighborhood) as
// they enter and leave the neighborhood.
func (s *syncService) discoverNeighborhood(ctx *context.T) {
	defer s.pending.Done()

	query := `v.InterfaceName="` + ifName + `"`
	ch, err := s.discovery.Scan(ctx, query)
	if err != nil {
		vlog.Errorf("sync: discoverNeighborhood: cannot start discovery service: %v", err)
		return
	}

	for !s.isClosed() {
		select {
		case update, ok := <-ch:
			if !ok || s.isClosed() {
				break
			}

			vlog.VI(3).Infof("discoverNeighborhood: got an update, %+v\n", update)

			if update.IsLost() {
				s.updateDiscoveryInfo(update.Id().String(), nil)
			} else {
				ad := update.Advertisement()
				s.updateDiscoveryInfo(update.Id().String(), &ad)
			}

		case <-s.closed:
			break
		}
	}

	vlog.VI(1).Info("sync: discoverNeighborhood: channel closed, stop listening and exit")
}

// updateDiscoveryInfo adds or removes information about a sync peer or a
// syncgroup found in the neighborhood through the discovery service.  If the
// service entry is nil the record is removed from its discovery map.  The peer
// and syncgroup information is stored in the service attributes.
func (s *syncService) updateDiscoveryInfo(id string, ad *discovery.Advertisement) {
	s.discoveryLock.Lock()
	defer s.discoveryLock.Unlock()

	vlog.VI(3).Infof("sync: updateDiscoveryInfo: %s: %+v, %p, current discoverySyncgroups is %+v", id, ad, ad, s.discoverySyncgroups)

	// The first time around initialize all discovery maps.
	if s.discoveryIds == nil {
		s.discoveryIds = make(map[string]*discovery.Advertisement)
		s.discoveryPeers = make(map[string]*discovery.Advertisement)
		s.discoverySyncgroups = make(map[interfaces.GroupId]map[string]*discovery.Advertisement)
	}

	// Determine the service entry type (sync peer or syncgroup) and its
	// value either from the given service info or previously stored entry.
	// Note: each entry only contains a single attribute.
	var attrs discovery.Attributes
	if ad != nil {
		attrs = ad.Attributes
	} else if serv := s.discoveryIds[id]; serv != nil {
		attrs = serv.Attributes
	}

	// handle peers
	if peer, ok := attrs[wire.DiscoveryAttrPeer]; ok {
		// The attribute value is the Syncbase peer name.
		if ad != nil {
			s.discoveryPeers[peer] = ad
		} else {
			delete(s.discoveryPeers, peer)
		}
	}

	// handle syngroups
	if sgName, ok := attrs[wire.DiscoveryAttrSyncgroupName]; ok {
		// The attribute value is the syncgroup name.
		dbId := wire.Id{Name: attrs[wire.DiscoveryAttrDatabaseName], Blessing: attrs[wire.DiscoveryAttrDatabaseBlessing]}
		sgId := wire.Id{Name: sgName, Blessing: attrs[wire.DiscoveryAttrSyncgroupBlessing]}
		gid := SgIdToGid(dbId, sgId)
		admins := s.discoverySyncgroups[gid]
		if ad != nil {
			if admins == nil {
				admins = make(map[string]*discovery.Advertisement)
				s.discoverySyncgroups[gid] = admins
				vlog.VI(3).Infof("added advertisement %+v, %p as dbId %v, sgId %v, gid %v\n", admins, admins, dbId, sgId, gid)

			}
			admins[id] = ad
		} else if admins != nil {
			delete(admins, id)
			if len(admins) == 0 {
				delete(s.discoverySyncgroups, gid)
				vlog.VI(3).Infof("deleted advertisement %+v, %p from dbId %v, sgId %v, gid %v\n", admins, admins, dbId, sgId, gid)
			}
		}
	}

	// Add or remove the service entry from the main IDs map.
	if ad != nil {
		s.discoveryIds[id] = ad
	} else {
		delete(s.discoveryIds, id)
	}
}

// filterDiscoveryPeers returns only those peers discovered via neighborhood
// that are also found in sgMembers (passed as input argument).
func (s *syncService) filterDiscoveryPeers(sgMembers map[string]uint32) map[string]*discovery.Advertisement {
	s.discoveryLock.Lock()
	defer s.discoveryLock.Unlock()

	if s.discoveryPeers == nil {
		return nil
	}

	sgNeighbors := make(map[string]*discovery.Advertisement)

	for peer, svc := range s.discoveryPeers {
		if _, ok := sgMembers[peer]; ok {
			sgNeighbors[peer] = svc
		}
	}

	return sgNeighbors
}

// filterSyncgroupAdmins returns syncgroup admins for the specified syncgroup
// found in the neighborhood via the discovery service.
func (s *syncService) filterSyncgroupAdmins(dbId, sgId wire.Id) []*discovery.Advertisement {
	s.discoveryLock.Lock()
	defer s.discoveryLock.Unlock()

	if s.discoverySyncgroups == nil {
		return nil
	}

	sgInfo := s.discoverySyncgroups[SgIdToGid(dbId, sgId)]
	if sgInfo == nil {
		return nil
	}

	admins := make([]*discovery.Advertisement, 0, len(sgInfo))
	for _, svc := range sgInfo {
		admins = append(admins, svc)
	}

	return admins
}

// AddNames publishes all the names for this Syncbase instance gathered from all
// the syncgroups it is currently participating in. This is needed when
// syncbased is restarted so that it can republish itself at the names being
// used in the syncgroups.
func AddNames(ctx *context.T, ss interfaces.SyncServerMethods, svr rpc.Server) error {
	vlog.VI(2).Infof("sync: AddNames: begin")
	defer vlog.VI(2).Infof("sync: AddNames: end")

	s := ss.(*syncService)
	s.nameLock.Lock()
	defer s.nameLock.Unlock()

	s.svr = svr

	info := s.copyMemberInfo(ctx, s.name)
	if info == nil || len(info.mtTables) == 0 {
		vlog.VI(2).Infof("sync: AddNames: end returning no names")
		return nil
	}

	for mt := range info.mtTables {
		name := naming.Join(mt, s.name)
		// Note that AddName will retry the publishing if not
		// successful. So if a node is offline, it will publish the name
		// when possible.
		if err := svr.AddName(name); err != nil {
			vlog.VI(2).Infof("sync: AddNames: end returning AddName err %v", err)
			return err
		}
	}
	if err := s.advertiseSyncbaseInNeighborhood(); err != nil {
		vlog.VI(2).Infof("sync: AddNames: end returning advertiseSyncbaseInNeighborhood err %v", err)
		return err
	}

	// Advertise syncgroups.
	for dbId, dbInfo := range info.db2sg {
		st, err := s.getDbStore(ctx, nil, dbId)
		if err != nil {
			return err
		}
		for gid := range dbInfo {
			sg, err := getSyncgroupByGid(ctx, st, gid)
			if err != nil {
				return err
			}
			if err := s.advertiseSyncgroupInNeighborhood(sg); err != nil {
				vlog.VI(2).Infof("sync: AddNames: end returning advertiseSyncgroupInNeighborhood err %v", err)
				return err
			}
		}
	}
	return nil
}

// advertiseSyncbaseInNeighborhood checks if the Syncbase service is already
// being advertised over the neighborhood. If not, it begins advertising. The
// caller of the function is holding nameLock.
func (s *syncService) advertiseSyncbaseInNeighborhood() error {
	if !s.publishInNh {
		return nil
	}

	vlog.VI(4).Infof("sync: advertiseSyncbaseInNeighborhood: begin")
	defer vlog.VI(4).Infof("sync: advertiseSyncbaseInNeighborhood: end")

	s.advLock.Lock()
	defer s.advLock.Unlock()
	// Syncbase is already being advertised.
	if s.cancelAdvSyncbase != nil {
		return nil
	}

	sbService := discovery.Advertisement{
		InterfaceName: ifName,
		Attributes: discovery.Attributes{
			wire.DiscoveryAttrPeer: s.name,
		},
	}

	// Note that duplicate calls to advertise will return an error.
	ctx, stop := context.WithCancel(s.ctx)
	ch, err := idiscovery.AdvertiseServer(ctx, s.discovery, s.svr, "", &sbService, nil)
	if err != nil {
		stop()
		return err
	}
	vlog.VI(4).Infof("sync: advertiseSyncbaseInNeighborhood: successful")
	s.cancelAdvSyncbase = func() {
		stop()
		<-ch
	}
	return nil
}

func (s *syncService) stopAdvertisingSyncbaseInNeighborhood() {
	s.advLock.Lock()
	cancelAdvSyncbase := s.cancelAdvSyncbase
	s.advLock.Unlock()
	if cancelAdvSyncbase != nil {
		cancelAdvSyncbase()
	}
}

// advertiseSyncgroupInNeighborhood checks if this Syncbase is an admin of the
// syncgroup, and if so advertises the syncgroup over neighborhood. If the
// Syncbase loses admin access, any previous syncgroup advertisements are
// cancelled. The caller of the function is holding nameLock.
func (s *syncService) advertiseSyncgroupInNeighborhood(sg *interfaces.Syncgroup) error {
	if !s.publishInNh {
		return nil
	}
	vlog.VI(4).Infof("sync: advertiseSyncgroupInNeighborhood: begin")
	defer vlog.VI(4).Infof("sync: advertiseSyncgroupInNeighborhood: end")

	s.advLock.Lock()
	defer s.advLock.Unlock()

	// Refresh the sg spec before advertising.  This prevents trampling
	// when concurrent spec updates apply transactions in one ordering,
	// but advertise in another.
	gid := SgIdToGid(sg.DbId, sg.Id)
	st, err := s.getDbStore(s.ctx, nil, sg.DbId)
	if err != nil {
		return err
	}
	sg, err = getSyncgroupByGid(s.ctx, st, gid)
	if err != nil {
		return err
	}

	state, advertising := s.advSyncgroups[gid]
	if advertising {
		// The spec hasn't changed since the last advertisement.
		if sg.SpecVersion == state.specVersion {
			return nil
		}
		state.cancel()
		delete(s.advSyncgroups, gid)
	}

	// We only advertise if potential members could join by contacting us.
	// For that reason we only advertise if we are an admin.
	if !syncgroupAdmin(s.ctx, sg.Spec.Perms) {
		return nil
	}

	sbService := discovery.Advertisement{
		InterfaceName: ifName,
		Attributes: discovery.Attributes{
			wire.DiscoveryAttrDatabaseName:      sg.DbId.Name,
			wire.DiscoveryAttrDatabaseBlessing:  sg.DbId.Blessing,
			wire.DiscoveryAttrSyncgroupName:     sg.Id.Name,
			wire.DiscoveryAttrSyncgroupBlessing: sg.Id.Blessing,
		},
	}
	if advertising {
		sbService.Id = state.adId
	}

	vlog.VI(4).Infof("sync: advertiseSyncgroupInNeighborhood: advertising %v", sbService)

	// TODO(mattr): Unfortunately, discovery visibility isn't as powerful
	// as an ACL.  There's no way to represent the NotIn list.  For now
	// if you match the In list you can see the advertisement, though you
	// might not be able to join.
	visibility := sg.Spec.Perms[string(access.Read)].In
	ctx, stop := context.WithCancel(s.ctx)
	ch, err := idiscovery.AdvertiseServer(ctx, s.discovery, s.svr, "", &sbService, visibility)
	if err != nil {
		stop()
		return err
	}
	vlog.VI(4).Infof("sync: advertiseSyncgroupInNeighborhood: successful")
	cancel := func() {
		stop()
		<-ch
	}
	s.advSyncgroups[gid] = syncAdvertisementState{cancel: cancel, specVersion: sg.SpecVersion, adId: sbService.Id}
	return nil
}

//////////////////////////////
// Helpers.

// isClosed returns true if the sync service channel is closed indicating that
// the service is shutting down.
func (s *syncService) isClosed() bool {
	select {
	case <-s.closed:
		return true
	default:
		return false
	}
}

// rand64 generates an unsigned 64-bit pseudo-random number.
func (s *syncService) rand64() uint64 {
	s.rngLock.Lock()
	defer s.rngLock.Unlock()
	return (uint64(s.rng.Int63()) << 1) | uint64(s.rng.Int63n(2))
}

// randIntn mimics rand.Intn (generates a non-negative pseudo-random number in [0,n)).
func (s *syncService) randIntn(n int) int {
	s.rngLock.Lock()
	defer s.rngLock.Unlock()
	return s.rng.Intn(n)
}

func syncbaseIdToName(id uint64) string {
	return fmt.Sprintf("%x", id)
}

func (s *syncService) stKey() string {
	return common.SyncPrefix
}

var (
	statMu  sync.Mutex
	statIdx int
)

func syncServiceStatName() string {
	statMu.Lock()
	ret := naming.Join("syncbase", "vsync", fmt.Sprint(statIdx))
	statIdx++
	statMu.Unlock()
	return ret
}

func (s *syncService) exportStats() {
	stats.NewStringFunc(s.statPrefix, func() string {
		s.discoveryLock.Lock()
		defer s.discoveryLock.Unlock()
		return fmt.Sprintf(`
Peers:        %v
Ads:          %v
Syncgroups:   %v
`, adMapKeys(s.discoveryPeers), adMapKeys(s.discoveryIds), groupMapKeys(s.discoverySyncgroups))
	})
}

func adMapKeys(m map[string]*discovery.Advertisement) []string {
	var ret []string
	for k, v := range m {
		ret = append(ret, fmt.Sprintf("%v: %v\n", k, *v))
	}
	return ret
}

func groupMapKeys(m map[interfaces.GroupId]map[string]*discovery.Advertisement) []string {
	var ret []string
	for k := range m {
		ret = append(ret, fmt.Sprint(k))
	}
	return ret
}

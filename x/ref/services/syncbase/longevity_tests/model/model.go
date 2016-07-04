// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package model defines functions for generating random sets of model
// databases, devices, and users that will be simulated in a syncbase longevity
// test.
package model

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"v.io/v23/conventions"
	"v.io/v23/security"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func randomName(prefix string) string {
	return fmt.Sprintf("%s-%08x", prefix, rand.Int31())
}

// ===========
// Permissions
// ===========

// Permissions maps tags to Users who are included on that tag.
// TODO(nlacasse): Consider allowing blacklisted users with "NotIn" lists.
type Permissions map[string]UserSet

func (perms Permissions) ToWire(rootBlessing string) access.Permissions {
	p := access.Permissions{}
	for tag, users := range perms {
		for _, user := range users {
			userBlessing := security.BlessingPattern(user.Name)
			if rootBlessing != "" {
				parsedBlessing := conventions.Blessing{
					IdentityProvider: rootBlessing,
					User:             user.Name,
				}
				userBlessing = parsedBlessing.UserPattern()
			}
			p.Add(userBlessing, tag)
		}
	}
	return p
}

func (perms Permissions) FilterTags(allowTags ...access.Tag) Permissions {
	filtered := Permissions{}
	for _, tag := range allowTags {
		if users, ok := perms[string(tag)]; ok {
			filtered[string(tag)] = users
		}
	}
	return filtered
}

// ===========
// Collections
// ===========

// Collection represents a syncbase collection.
// TODO(nlacasse): Put syncgroups in here?  It's a bit tricky because they need
// to have a host device name in their name, and the spec depends on the
// mounttable.
type Collection struct {
	// Name of the collection.
	Name string
	// Blessing of the collection.
	Blessing string
	// Permissions of the collection.
	Permissions Permissions
}

func (c *Collection) String() string {
	return fmt.Sprintf("{collection %v blessing=%v}", c.Name, c.Blessing)
}

func (c *Collection) Id() wire.Id {
	return wire.Id{
		Name:     c.Name,
		Blessing: c.Blessing,
	}
}

type CollectionSet []Collection

func (cols CollectionSet) String() string {
	strs := []string{}
	for _, col := range cols {
		strs = append(strs, col.String())
	}
	return fmt.Sprintf("[%s]", strings.Join(strs, ", "))
}

// ==========
// Syncgroups
// ==========

// Syncgroup represents a syncgroup.
type Syncgroup struct {
	HostDevice  *Device
	Name        string
	Blessing    string
	Description string
	Collections []Collection
	Permissions Permissions
	IsPrivate   bool
	// Devices which will attempt to create the syncgroup.
	CreatorDevices DeviceSet
	// Devices which will attempt to join the syncgroup.
	JoinerDevices DeviceSet
}

func (sg *Syncgroup) String() string {
	// TODO(ivanpi): Include more info about syncgroup.
	return fmt.Sprintf("{syncgroup %v blessing=%v}", sg.Name, sg.Blessing)
}

func (sg *Syncgroup) Id() wire.Id {
	return wire.Id{
		Name:     sg.Name,
		Blessing: sg.Blessing,
	}
}

func (sg *Syncgroup) Spec(rootBlessing string) wire.SyncgroupSpec {
	collections := make([]wire.Id, len(sg.Collections))

	for i, col := range sg.Collections {
		collections[i] = col.Id()
	}

	return wire.SyncgroupSpec{
		Description: sg.Description,
		Collections: collections,
		Perms:       sg.Permissions.ToWire(rootBlessing),
		IsPrivate:   sg.IsPrivate,
	}
}

type SyncgroupSet []Syncgroup

func (sgs SyncgroupSet) String() string {
	strs := []string{}
	for _, sg := range sgs {
		strs = append(strs, sg.String())
	}
	return fmt.Sprintf("[%s]", strings.Join(strs, ", "))
}

// =========
// Databases
// =========

// Database represents a syncbase database.  Each database corresponds to a
// single app.
type Database struct {
	// Name of the database.
	Name string
	// Blessing of the database.
	Blessing string
	// Collections.
	Collections CollectionSet
	// Syncgroups.
	Syncgroups SyncgroupSet
	// Permissions.
	Permissions Permissions
}

func (db *Database) String() string {
	return fmt.Sprintf("{database %v blessing=%v collections=%v syncgroups=%v perms=%v}", db.Name, db.Blessing, db.Collections, db.Syncgroups, db.Permissions.ToWire(""))
}

func (db *Database) Id() wire.Id {
	return wire.Id{
		Name:     db.Name,
		Blessing: db.Blessing,
	}
}

// DatabaseSet represents a set of Databases.
// TODO(nlacasse): Consider using a map here if uniqueness becomes an issue.
type DatabaseSet []*Database

// unique removes all duplicate entries from the DatabaseSet.
// TODO(nlacasse): Uniqueness is becoming an issue.  Make all Sets into maps.
func (dbs DatabaseSet) unique() DatabaseSet {
	found := map[*Database]struct{}{}
	udbs := DatabaseSet{}
	for _, db := range dbs {
		if _, ok := found[db]; !ok {
			udbs = append(udbs, db)
			found[db] = struct{}{}
		}
	}
	return udbs
}

// GenerateDatabaseSet generates a DatabaseSet with n databases.
// TODO(nlacasse): Generate collections and syncgroups.
// TODO(nlacasse): Generate clients.
func GenerateDatabaseSet(n int) DatabaseSet {
	dbs := DatabaseSet{}
	for i := 0; i < n; i++ {
		db := &Database{
			Name:     randomName("db"),
			Blessing: string(security.AllPrincipals),
		}
		dbs = append(dbs, db)
	}
	return dbs
}

// RandomSubset returns a random subset of the DatabaseSet with at least min
// and at most max databases.
func (dbs DatabaseSet) RandomSubset(min, max int) DatabaseSet {
	if min < 0 || min > len(dbs) || min > max {
		panic(fmt.Errorf("invalid arguments to RandomSubset: min=%v max=%v len(dbs)=%v", min, max, len(dbs)))
	}
	if max > len(dbs) {
		max = len(dbs)
	}
	n := min + rand.Intn(max-min+1)
	subset := make(DatabaseSet, n)
	for i, j := range rand.Perm(len(dbs))[:n] {
		subset[i] = dbs[j]
	}
	return subset
}

func (dbs DatabaseSet) String() string {
	r := make([]string, len(dbs))
	for i, j := range dbs {
		r[i] = j.Name
	}
	return fmt.Sprintf("[%s]", strings.Join(r, ", "))
}

// =======
// Devices
// =======

// ConnectivitySpec specifies the network connectivity of the device.
type ConnectivitySpec string

const (
	Online  ConnectivitySpec = "online"
	Offline ConnectivitySpec = "offline"
	// TODO(nlacasse): Add specs for latency, bandwidth, etc.
)

// DeviceSpec specifies the possible types of devices.
type DeviceSpec struct {
	// Maximum number of databases allowed on the device.
	MaxDatabases int
	// Types of connectivity allowed for the database.
	ConnectivitySpecs []ConnectivitySpec
}

// Some common DeviceSpecs.  It would be nice if these were consts, but go won't allow that.
var (
	LaptopSpec = DeviceSpec{
		MaxDatabases:      10,
		ConnectivitySpecs: []ConnectivitySpec{"online", "offline"},
	}

	PhoneSpec = DeviceSpec{
		MaxDatabases:      5,
		ConnectivitySpecs: []ConnectivitySpec{"online", "offline"},
	}

	CloudSpec = DeviceSpec{
		MaxDatabases:      100,
		ConnectivitySpecs: []ConnectivitySpec{"online"},
	}
	// TODO(nlacasse): Add more DeviceSpecs for tablet, desktop, camera,
	// wearable, etc.
)

// Device represents a device.
type Device struct {
	// Name of the device.
	Name string
	// Databases inluded on the device.
	Databases DatabaseSet
	// The device's spec.
	Spec DeviceSpec
	// Current connectivity spec for the device.  This value must be included
	// in Spec.ConnectivitySpecs.
	CurrentConnectivity ConnectivitySpec
	// Associated clients that will operate on this syncbase instance.  Clients
	// must be registered with control.RegisterClient.
	Clients []string
}

func (d Device) String() string {
	return fmt.Sprintf("{device %v databases=%v clients=%v}", d.Name, d.Databases, d.Clients)
}

// DeviceSet is a set of devices.
// TODO(nlacasse): Consider using a map here if uniqueness becomes an issue.
type DeviceSet []*Device

// GenerateDeviceSet generates a device set of size n.  The device spec for
// each device is chosen randomly from specs, and each device is given a
// non-empty set of databases taken from databases argument, obeying the maxium
// for the device spec.
func GenerateDeviceSet(n int, databases DatabaseSet, specs []DeviceSpec) DeviceSet {
	ds := DeviceSet{}
	for i := 0; i < n; i++ {
		// Pick a random spec.
		idx := rand.Intn(len(specs))
		spec := specs[idx]
		databases := databases.RandomSubset(1, spec.MaxDatabases)

		d := &Device{
			Name:      randomName("device"),
			Databases: databases,
			Spec:      spec,
			// Pick a random ConnectivitySpec from those allowed by the
			// DeviceSpec.
			CurrentConnectivity: spec.ConnectivitySpecs[rand.Intn(len(spec.ConnectivitySpecs))],
		}
		ds = append(ds, d)
	}
	return ds
}

func (ds DeviceSet) String() string {
	r := make([]string, len(ds))
	for i, j := range ds {
		r[i] = j.Name
	}
	return fmt.Sprintf("[%s]", strings.Join(r, ", "))
}

// Lookup returns the first Device in the DeviceSet with the given name, or nil
// if none exists.
func (ds DeviceSet) Lookup(name string) *Device {
	for _, d := range ds {
		if d.Name == name {
			return d
		}
	}
	return nil
}

// Topology is an adjacency matrix specifying the connection type between
// devices.
// TODO(nlacasse): For now we only specify which devices are reachable by each
// device.
type Topology map[*Device]DeviceSet

func (top Topology) String() string {
	str := ""
	for d, ds := range top {
		str += fmt.Sprintf("%v can send to %v\n", d.Name, ds)
	}
	return str
}

// GenerateTopology generates a Topology on the given DeviceSet.  The affinity
// argument specifies the probability that any two devices are connected.  We
// ensure that the generated topology is symmetric, but we do *not* guarantee
// that it is connected.
// TODO(nlacasse): Have more fine-grained affinity taking into account the
// DeviceSpec of each device.  E.g. Desktop is likely to be connected to the
// cloud, but less likely to be connected to a watch.
func GenerateTopology(devices DeviceSet, affinity float64) Topology {
	top := Topology{}
	for i, d1 := range devices {
		// All devices are connected to themselves.
		top[d1] = append(top[d1], d1)
		for _, d2 := range devices[i+1:] {
			connected := rand.Float64() <= affinity
			if connected {
				top[d1] = append(top[d1], d2)
				top[d2] = append(top[d2], d1)
			}
		}
	}
	return top
}

// =====
// Users
// =====

// User represents a user.
type User struct {
	// The user's name.
	Name string
	// The user's devices.  All databases in the user's devices will be
	// included in the user's Databases.
	Devices DeviceSet
}

// Databases returns all databases contained on the user's devices.
func (user *User) Databases() DatabaseSet {
	dbs := DatabaseSet{}
	for _, dev := range user.Devices {
		dbs = append(dbs, dev.Databases...)
	}
	return dbs.unique()
}

func (user *User) String() string {
	return fmt.Sprintf(`{user %v devices=%v}`, user.Name, user.Devices)
}

// UserSet is a set of users.
type UserSet []*User

// UserOpts specifies the options to use when creating a random user.
type UserOpts struct {
	MaxDatabases int
	MaxDevices   int
	MinDevices   int
}

// GenerateUser generates a random user with nonempty set of databases from the
// given database set and n devices.
// TODO(nlacasse): Should DatabaseSet be in UserOpts?
func GenerateUser(dbs DatabaseSet, opts UserOpts) *User {
	// TODO(nlacasse): Make this a parameter?
	specs := []DeviceSpec{
		LaptopSpec,
		PhoneSpec,
		CloudSpec,
	}

	databases := dbs.RandomSubset(1, opts.MaxDatabases)
	numDevices := opts.MinDevices
	if opts.MaxDevices > opts.MinDevices {
		numDevices += rand.Intn(opts.MaxDevices - opts.MinDevices)
	}
	devices := GenerateDeviceSet(numDevices, databases, specs)

	return &User{
		Name:    randomName("user"),
		Devices: devices,
	}
}

// ========
// Universe
// ========

type Universe struct {
	// All users in the universe.
	Users UserSet
	// Description of device connectivity.
	Topology Topology
}

// Databases returns all databases contained in the universe.
func (u *Universe) Databases() DatabaseSet {
	dbs := DatabaseSet{}
	for _, user := range u.Users {
		dbs = append(dbs, user.Databases()...)
	}
	return dbs.unique()
}

func (u *Universe) String() string {
	// Collect all device and user descriptions with a newline in between each.
	devStrings := []string{}
	userStrings := []string{}
	for _, user := range u.Users {
		userStrings = append(userStrings, user.String())
		for _, dev := range user.Devices {
			devStrings = append(devStrings, dev.String())
		}
	}
	devString := strings.Join(devStrings, "\n")
	userString := strings.Join(userStrings, "\n")

	dbStrings := []string{}
	for _, db := range u.Databases() {
		dbStrings = append(dbStrings, db.String())
	}
	dbString := strings.Join(dbStrings, "\n")

	return fmt.Sprintf(`Databases:
%v

Devices:
%v

Users:
%v

Topology:
%v`, dbString, devString, userString, u.Topology)
}

// UniverseOpts specifies the options to use when creating a random universe.
type UniverseOpts struct {
	// Probability that any two devices are connected
	DeviceAffinity float64
	// Maximum number of databases in the universe.
	MaxDatabases int
	// Number of users in the universe.
	NumUsers int
	// Maximum number of databases for any user.
	MaxDatabasesPerUser int
	// Maximum number of devices for any user.
	MaxDevicesPerUser int
	// Minimum number of devices for any user.
	MinDevicesPerUser int
}

func GenerateUniverse(opts UniverseOpts) Universe {
	dbs := GenerateDatabaseSet(opts.MaxDatabases)
	userOpts := UserOpts{
		MaxDatabases: opts.MaxDatabasesPerUser,
		MaxDevices:   opts.MaxDevicesPerUser,
		MinDevices:   opts.MinDevicesPerUser,
	}
	users := UserSet{}
	devices := DeviceSet{}
	for i := 0; i < opts.NumUsers; i++ {
		user := GenerateUser(dbs, userOpts)
		users = append(users, user)
		devices = append(devices, user.Devices...)
	}

	return Universe{
		Users:    users,
		Topology: GenerateTopology(devices, opts.DeviceAffinity),
	}
}

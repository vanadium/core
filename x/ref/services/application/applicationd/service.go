// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"sort"
	"strings"

	"v.io/v23/context"
	"v.io/v23/glob"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/services/application"
	"v.io/v23/verror"
	"v.io/x/lib/set"
	"v.io/x/ref/services/internal/fs"
	"v.io/x/ref/services/internal/pathperms"
	"v.io/x/ref/services/repository"
)

// appRepoService implements the Application repository interface.
type appRepoService struct {
	// store is the storage server used for storing application
	// metadata.
	// All objects share the same Memstore.
	store *fs.Memstore
	// storeRoot is a name in the directory under which all data will be
	// stored.
	storeRoot string
	// suffix is the name of the application object.
	suffix string
}

const pkgPath = "v.io/x/ref/services/application/applicationd"

var (
	ErrInvalidSuffix   = verror.Register(pkgPath+".InvalidSuffix", verror.NoRetry, "{1:}{2:} invalid suffix{:_}")
	ErrOperationFailed = verror.Register(pkgPath+".OperationFailed", verror.NoRetry, "{1:}{2:} operation failed{:_}")
	ErrNotAuthorized   = verror.Register(pkgPath+".errNotAuthorized", verror.NoRetry, "{1:}{2:} none of the client's blessings are valid {:_}")
)

// NewApplicationService returns a new Application service implementation.
func NewApplicationService(store *fs.Memstore, storeRoot, suffix string) repository.ApplicationServerMethods {
	return &appRepoService{store: store, storeRoot: storeRoot, suffix: suffix}
}

func parse(ctx *context.T, suffix string) (string, string, error) {
	tokens := strings.Split(suffix, "/")
	switch len(tokens) {
	case 2:
		return tokens[0], tokens[1], nil
	case 1:
		return tokens[0], "", nil
	default:
		return "", "", verror.New(ErrInvalidSuffix, ctx)
	}
}

func (i *appRepoService) Profiles(ctx *context.T, call rpc.ServerCall) ([]string, error) {
	ctx.VI(0).Infof("%v.Profiles()", i.suffix)
	name, version, err := parse(ctx, i.suffix)
	if err != nil {
		return []string{}, err
	}
	i.store.Lock()
	defer i.store.Unlock()

	profiles, err := i.store.BindObject(naming.Join("/applications", name)).Children()
	if err != nil {
		return []string{}, err
	}
	if version == "" {
		return profiles, nil
	}
	profilesRet := make(map[string]struct{})
	for _, profile := range profiles {
		versions, err := i.store.BindObject(naming.Join("/applications", name, profile)).Children()
		if err != nil {
			return []string{}, err
		}
		for _, v := range versions {
			if version == v {
				profilesRet[profile] = struct{}{}
				break
			}
		}
	}
	ret := set.String.ToSlice(profilesRet)
	if len(ret) == 0 {
		return []string{}, verror.New(verror.ErrNoExist, ctx)
	}
	sort.Strings(ret)
	return ret, nil
}

func (i *appRepoService) Match(ctx *context.T, call rpc.ServerCall, profiles []string) (application.Envelope, error) {
	ctx.VI(0).Infof("%v.Match(%v)", i.suffix, profiles)
	empty := application.Envelope{}
	name, version, err := parse(ctx, i.suffix)
	if err != nil {
		return empty, err
	}

	i.store.Lock()
	defer i.store.Unlock()

	if version == "" {
		versions, err := i.allAppVersionsForProfiles(name, profiles)
		if err != nil {
			return empty, err
		}
		if len(versions) < 1 {
			return empty, verror.New(verror.ErrNoExist, ctx)
		}
		sort.Strings(versions)
		version = versions[len(versions)-1]
	}

	for _, profile := range profiles {
		path := naming.Join("/applications", name, profile, version)
		entry, err := i.store.BindObject(path).Get(call)
		if err != nil {
			continue
		}
		envelope, ok := entry.Value.(application.Envelope)
		if !ok {
			continue
		}
		return envelope, nil
	}
	return empty, verror.New(verror.ErrNoExist, ctx)
}

func (i *appRepoService) Put(ctx *context.T, call rpc.ServerCall, profile string, envelope application.Envelope, overwrite bool) error {
	ctx.VI(0).Infof("%v.Put(%v, %v, %t)", i.suffix, profile, envelope, overwrite)
	name, version, err := parse(ctx, i.suffix)
	if err != nil {
		return err
	}
	if version == "" {
		return verror.New(ErrInvalidSuffix, ctx)
	}
	i.store.Lock()
	defer i.store.Unlock()
	// Transaction is rooted at "", so tname == tid.
	tname, err := i.store.BindTransactionRoot("").CreateTransaction(call)
	if err != nil {
		return err
	}

	// Only add a Permissions value if there is not already one present.
	apath := naming.Join("/acls", name, "data")
	aobj := i.store.BindObject(apath)
	if _, err := aobj.Get(call); verror.ErrorID(err) == fs.ErrNotInMemStore.ID {
		rb, _ := security.RemoteBlessingNames(ctx, call.Security())
		if len(rb) == 0 {
			// None of the client's blessings are valid.
			return verror.New(ErrNotAuthorized, ctx)
		}
		newperms := pathperms.PermissionsForBlessings(rb)
		if _, err := aobj.Put(nil, newperms); err != nil {
			return err
		}
	}

	path := naming.Join(tname, "/applications", name, profile, version)
	object := i.store.BindObject(path)
	if _, err := object.Get(call); verror.ErrorID(err) != fs.ErrNotInMemStore.ID && !overwrite {
		return verror.New(verror.ErrExist, ctx, "envelope already exists for profile", profile)
	}
	if _, err := object.Put(call, envelope); err != nil {
		return verror.New(ErrOperationFailed, ctx)
	}
	if err := i.store.BindTransaction(tname).Commit(call); err != nil {
		return verror.New(ErrOperationFailed, ctx)
	}
	return nil
}

func (i *appRepoService) Remove(ctx *context.T, call rpc.ServerCall, profile string) error {
	ctx.VI(0).Infof("%v.Remove(%v)", i.suffix, profile)
	name, version, err := parse(ctx, i.suffix)
	if err != nil {
		return err
	}
	i.store.Lock()
	defer i.store.Unlock()
	// Transaction is rooted at "", so tname == tid.
	tname, err := i.store.BindTransactionRoot("").CreateTransaction(call)
	if err != nil {
		return err
	}
	profiles := []string{profile}
	if profile == "*" {
		var err error
		if profiles, err = i.store.BindObject(naming.Join("/applications", name)).Children(); err != nil {
			return err
		}
	}
	for _, profile := range profiles {
		path := naming.Join(tname, "/applications", name, profile)
		if version != "" {
			path += "/" + version
		}
		object := i.store.BindObject(path)
		found, err := object.Exists(call)
		if err != nil {
			return verror.New(ErrOperationFailed, ctx)
		}
		if !found {
			return verror.New(verror.ErrNoExist, ctx)
		}
		if err := object.Remove(call); err != nil {
			return verror.New(ErrOperationFailed, ctx)
		}
	}
	if err := i.store.BindTransaction(tname).Commit(call); err != nil {
		return verror.New(ErrOperationFailed, ctx)
	}
	return nil
}

func (i *appRepoService) allApplications() ([]string, error) {
	apps, err := i.store.BindObject("/applications").Children()
	// There is no actual object corresponding to "/applications" in the
	// store, so Children() returns ErrNoExist when there are no actual app
	// objects under /applications.
	if verror.ErrorID(err) == verror.ErrNoExist.ID {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return apps, nil
}

func (i *appRepoService) allAppVersionsForProfiles(appName string, profiles []string) ([]string, error) {
	uniqueVersions := make(map[string]struct{})
	for _, profile := range profiles {
		versions, err := i.store.BindObject(naming.Join("/applications", appName, profile)).Children()
		if verror.ErrorID(err) == verror.ErrNoExist.ID {
			continue
		} else if err != nil {
			return nil, err
		}
		set.String.Union(uniqueVersions, set.String.FromSlice(versions))
	}
	return set.String.ToSlice(uniqueVersions), nil
}

func (i *appRepoService) allAppVersions(appName string) ([]string, error) {
	profiles, err := i.store.BindObject(naming.Join("/applications", appName)).Children()
	if err != nil {
		return nil, err
	}
	return i.allAppVersionsForProfiles(appName, profiles)
}

func (i *appRepoService) GlobChildren__(ctx *context.T, call rpc.GlobChildrenServerCall, m *glob.Element) error {
	ctx.VI(0).Infof("%v.GlobChildren__()", i.suffix)
	i.store.Lock()
	defer i.store.Unlock()

	var elems []string
	if i.suffix != "" {
		elems = strings.Split(i.suffix, "/")
	}

	var results []string
	var err error
	switch len(elems) {
	case 0:
		results, err = i.allApplications()
		if err != nil {
			return err
		}
	case 1:
		results, err = i.allAppVersions(elems[0])
		if err != nil {
			return err
		}
	case 2:
		versions, err := i.allAppVersions(elems[0])
		if err != nil {
			return err
		}
		for _, v := range versions {
			if v == elems[1] {
				return nil
			}
		}
		return verror.New(verror.ErrNoExist, nil)
	default:
		return verror.New(verror.ErrNoExist, nil)
	}

	for _, r := range results {
		if m.Match(r) {
			call.SendStream().Send(naming.GlobChildrenReplyName{Value: r})
		}
	}
	return nil
}

func (i *appRepoService) GetPermissions(ctx *context.T, call rpc.ServerCall) (perms access.Permissions, version string, err error) {
	name, _, err := parse(ctx, i.suffix)
	if err != nil {
		return nil, "", err
	}
	i.store.Lock()
	defer i.store.Unlock()
	path := naming.Join("/acls", name, "data")

	perms, version, err = getPermissions(ctx, i.store, path)
	if verror.ErrorID(err) == verror.ErrNoExist.ID {
		return pathperms.NilAuthPermissions(ctx, call.Security()), "", nil
	}

	return perms, version, err
}

func (i *appRepoService) SetPermissions(ctx *context.T, _ rpc.ServerCall, perms access.Permissions, version string) error {
	name, _, err := parse(ctx, i.suffix)
	if err != nil {
		return err
	}
	i.store.Lock()
	defer i.store.Unlock()
	path := naming.Join("/acls", name, "data")
	return setPermissions(ctx, i.store, path, perms, version)
}

// getPermissions fetches a Permissions out of the Memstore at the provided path.
// path is expected to already have been cleaned by naming.Join or its ilk.
func getPermissions(ctx *context.T, store *fs.Memstore, path string) (access.Permissions, string, error) {
	entry, err := store.BindObject(path).Get(nil)

	if verror.ErrorID(err) == fs.ErrNotInMemStore.ID {
		// No Permissions exists
		return nil, "", verror.New(verror.ErrNoExist, nil)
	} else if err != nil {
		ctx.Errorf("getPermissions: internal failure in fs.Memstore")
		return nil, "", err
	}

	perms, ok := entry.Value.(access.Permissions)
	if !ok {
		return nil, "", err
	}

	version, err := pathperms.ComputeVersion(perms)
	if err != nil {
		return nil, "", err
	}
	return perms, version, nil
}

// setPermissions writes a Permissions into the Memstore at the provided path.
// where path is expected to have already been cleaned by naming.Join.
func setPermissions(ctx *context.T, store *fs.Memstore, path string, perms access.Permissions, version string) error {
	if version != "" {
		_, oversion, err := getPermissions(ctx, store, path)
		if verror.ErrorID(err) == verror.ErrNoExist.ID {
			oversion = version
		} else if err != nil {
			return err
		}

		if oversion != version {
			return verror.NewErrBadVersion(nil)
		}
	}

	tname, err := store.BindTransactionRoot("").CreateTransaction(nil)
	if err != nil {
		return err
	}

	object := store.BindObject(path)

	if _, err := object.Put(nil, perms); err != nil {
		return err
	}
	if err := store.BindTransaction(tname).Commit(nil); err != nil {
		return verror.New(ErrOperationFailed, nil)
	}
	return nil
}

func (i *appRepoService) tidyRemoveVersions(call rpc.ServerCall, tname, appName, profile string, versions []string) error {
	for _, v := range versions {
		path := naming.Join(tname, "/applications", appName, profile, v)
		object := i.store.BindObject(path)
		if err := object.Remove(call); err != nil {
			return err
		}
	}
	return nil
}

// numberOfVersionsToKeep can be set for tests.
var numberOfVersionsToKeep = 5

func (i *appRepoService) TidyNow(ctx *context.T, call rpc.ServerCall) error {
	ctx.VI(2).Infof("%v.TidyNow()", i.suffix)
	i.store.Lock()
	defer i.store.Unlock()

	tname, err := i.store.BindTransactionRoot("").CreateTransaction(call)
	if err != nil {
		return err
	}

	apps, err := i.allApplications()
	if err != nil {
		return err
	}

	for _, app := range apps {
		profiles, err := i.store.BindObject(naming.Join("/applications", app)).Children()
		if err != nil {
			return err
		}

		for _, profile := range profiles {
			versions, err := i.store.BindObject(naming.Join("/applications", app, profile)).Children()
			if err != nil {
				return err
			}

			lv := len(versions)
			if lv <= numberOfVersionsToKeep {
				continue
			}

			// Per assumption in Match, version names should ascend.
			sort.Strings(versions)
			versionsToRemove := versions[0 : lv-numberOfVersionsToKeep]
			if err := i.tidyRemoveVersions(call, tname, app, profile, versionsToRemove); err != nil {
				return err
			}
		}
	}

	if err := i.store.BindTransaction(tname).Commit(call); err != nil {
		return verror.New(ErrOperationFailed, ctx)
	}
	return nil

}

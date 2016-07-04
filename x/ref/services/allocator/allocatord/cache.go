// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"sync"

	"github.com/golang/groupcache/lru"

	"v.io/v23/context"
	"v.io/v23/verror"
	"v.io/x/ref/services/allocator"
)

var (
	// instanceCache maps email to list of instances.
	instanceCache      *lru.Cache
	instanceCacheMutex sync.Mutex
)

func checkOwnerOfInstance(ctx *context.T, email, handle string) error {
	return checkOwner(ctx, email, func(instance allocator.Instance) bool {
		return instance.Handle == handle
	})
}

func checkOwnerOfMountName(ctx *context.T, email, mName string) error {
	return checkOwner(ctx, email, func(instance allocator.Instance) bool {
		return instance.MountName == mName
	})
}

// checkOwner returns the mount name of the instance if the given email has
// access to it, or an error otherwise.
func checkOwner(ctx *context.T, email string, predicate func(allocator.Instance) bool) error {
	instanceCacheMutex.Lock()
	if instanceCache == nil {
		instanceCache = lru.New(maxInstancesFlag)
	}
	v, ok := instanceCache.Get(email)
	instanceCacheMutex.Unlock()
	if ok {
		if instances, ok := v.([]allocator.Instance); !ok {
			// Our cache code is broken.  Proceed to refresh entry.
			ctx.Errorf("invalid cache entry type %T for email %v", v, email)
		} else {
			for _, instance := range instances {
				if predicate(instance) {
					return nil
				}
			}
		}
	}

	instances, err := serverInstances(ctx, email)
	if err != nil {
		return err
	}
	// Regardless of whether an instance matches or not, update the email's
	// cache entry since the user is using the system.
	instanceCacheMutex.Lock()
	instanceCache.Add(email, instances)
	instanceCacheMutex.Unlock()

	for _, instance := range instances {
		if predicate(instance) {
			return nil
		}
	}
	return verror.New(verror.ErrNoExistOrNoAccess, nil)
}

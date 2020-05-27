// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

import (
	"v.io/v23/naming"
	"v.io/v23/security"
	"v.io/v23/services/application"
)

// TP is a convenience function. It prepends the transactionNamePrefix
// to the given path.
func TP(path string) string {
	return naming.Join(transactionNamePrefix, path)
}

func (ms *Memstore) PersistedFile() string {
	return ms.persistedFile
}

func translateToGobEncodeable(in interface{}) interface{} {
	env, ok := in.(application.Envelope)
	if !ok {
		return in
	}
	return applicationEnvelope{
		Title:     env.Title,
		Args:      env.Args,
		Binary:    env.Binary,
		Publisher: security.MarshalBlessings(env.Publisher),
		Env:       env.Env,
		Packages:  env.Packages,
	}
}

func (ms *Memstore) GetGOBConvertedMemstore() map[string]interface{} {
	convertedMap := make(map[string]interface{})
	for k, v := range ms.data {
		if tv, ok := v.(application.Envelope); ok {
			convertedMap[k] = translateToGobEncodeable(tv)
		}
	}
	return convertedMap
}

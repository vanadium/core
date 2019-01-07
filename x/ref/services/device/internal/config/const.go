// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package config

const (
	// EnvelopeEnv is the name of the environment variable that holds the
	// serialized device manager application envelope.
	EnvelopeEnv = "V23_DM_ENVELOPE"
	// PreviousEnv is the name of the environment variable that holds the
	// path to the previous version of the device manager.
	PreviousEnv = "V23_DM_PREVIOUS"
	// OriginEnv is the name of the environment variable that holds the
	// object name of the application repository that can be used to
	// retrieve the device manager application envelope.
	OriginEnv = "V23_DM_ORIGIN"
	// RootEnv is the name of the environment variable that holds the
	// path to the directory in which device manager workspaces are
	// created.
	RootEnv = "V23_DM_ROOT"
	// CurrentLinkEnv is the name of the environment variable that holds
	// the path to the soft link that points to the current device manager.
	CurrentLinkEnv = "V23_DM_CURRENT"
	// HelperEnv is the name of the environment variable that holds the path
	// to the suid helper used to start apps as specific system users.
	HelperEnv = "V23_DM_HELPER"
)

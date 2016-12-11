// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package mgmt defines constants used by the management tools and daemons.
package mgmt

const (
	ParentNameConfigKey            = "MGMT_PARENT_PROCESS_NAME"
	ChildNameConfigKey             = "MGMT_CHILD_PROCESS_NAME"
	AppCycleManagerConfigKey       = "MGMT_APP_CYCLE_MANAGER_NAME"
	AddressConfigKey               = "MGMT_CHILD_PROCESS_ADDRESS"
	ProtocolConfigKey              = "MGMT_CHILD_PROCESS_PROTOCOL"
	SecurityAgentEndpointConfigKey = "MGMT_SECURITY_AGENT_EP"
	SecurityAgentPathConfigKey     = "MGMT_SECURITY_AGENT_PATH"
	AppOriginConfigKey             = "MGMT_APP_ORIGIN"
	PublisherBlessingPrefixesKey   = "MGMT_PUBLISHER_BLESSING_PREFIXES"
	InstanceNameKey                = "MGMT_INSTANCE_NAME"
	AppCycleBlessingsKey           = "MGMT_APP_CYCLE_BLESSINGS"
)

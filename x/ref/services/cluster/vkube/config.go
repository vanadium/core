// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

// The config file used by the vkube command.
type vkubeConfig struct {
	// The GCE project name.
	Project string `json:"project"`
	// The GCE zone.
	Zone string `json:"zone"`
	// The name of the Kubernetes cluster.
	Cluster string `json:"cluster"`

	ClusterAgent clusterAgentConfig `json:"clusterAgent,omitempty"`
	PodAgent     podAgentConfig     `json:"podAgent,omitempty"`
}

type clusterAgentConfig struct {
	// The Kubernetes namespace of the cluster agent. An empty
	// value is equivalent to "default".
	Namespace string `json:"namespace,omitempty"`
	// The name of the docker image for the cluster agent.
	Image string `json:"image"`
	// The amount of CPU to reserve for the cluster agent.
	CPU string `json:"cpu"`
	// The amount of memory to reserve for the cluster agent.
	Memory string `json:"memory"`
	// The blessing name of the cluster agent.
	Blessing string `json:"blessing"`
	// The blessing pattern of the cluster agent admin, i.e. who's
	// allowed to create and delete secrets.
	Admin string `json:"admin"`
	// The external IP address of the cluster agent. An empty value
	// means that an ephemeral address will be used.
	ExternalIP string `json:"externalIP,omitempty"`
	// The name of the GCE Persistent Disk of the cluster agent. An empty
	// value means that the cluster agent won't use a GCE persistent disk.
	// PersistentDisk and PersistentVolumeClaim are mutually exclusive.
	PersistentDisk string `json:"persistentDisk,omitempty"`
	// The name of the Persistent Volume Claim of the cluster agent. An
	// empty value means that the cluster agent won't use a persistent
	// volume claim.
	PersistentVolumeClaim string `json:"persistentVolumeClaim,omitempty"`
	// When true, indicates that the cluster agent should only have an
	// IP address that is reachable from within the kubernetes cluster.
	InternalOnly bool `json:"internalOnly,omitempty"`
}

type podAgentConfig struct {
	// The name of the docker image for the pod agent.
	Image string `json:"image,omitempty"`
}

// readConfig reads a config file.
func readConfig(fileName string) (*vkubeConfig, error) {
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	var config vkubeConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %v", err)
	}
	if config.ClusterAgent.Namespace == "" {
		config.ClusterAgent.Namespace = "default"
	}
	return &config, nil
}

// writeConfig writes a config file.
func writeConfig(fileName string, config *vkubeConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	f, err := ioutil.TempFile(filepath.Dir(fileName), ".vkube-cfg-")
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), fileName); err != nil {
		return err
	}
	return nil
}

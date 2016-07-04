// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"

	"v.io/v23/context"
	"v.io/v23/conventions"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/x/ref/services/allocator"
)

const pkgPath = "v.io/x/ref/services/allocator/allocatord"

var (
	errLimitExceeded       = verror.Register(pkgPath+".errLimitExceeded", verror.NoRetry, "{1:}{2:} limit ({3}) exceeded")
	errGlobalLimitExceeded = verror.Register(pkgPath+".errGlobalLimitExceeded", verror.NoRetry, "{1:}{2:} global limit ({3}) exceeded")
)

type allocatorImpl struct {
	baseBlessings     security.Blessings
	baseBlessingNames []string
}

// Create creates a new instance of the service.
// It returns a handle for the new instance.
func (i *allocatorImpl) Create(ctx *context.T, call rpc.ServerCall) (string, error) {
	b, _ := security.RemoteBlessingNames(ctx, call.Security())
	ctx.Infof("Create() called by %v", b)

	email := emailFromBlessingNames(b)
	if email == "" {
		return "", verror.New(verror.ErrNoAccess, ctx, "unable to determine caller's email address")
	}
	return create(ctx, email, i.baseBlessings, i.baseBlessingNames)
}

// Destroy destroys the instance with the given handle.
func (i *allocatorImpl) Destroy(ctx *context.T, call rpc.ServerCall, kName string) error {
	b, _ := security.RemoteBlessingNames(ctx, call.Security())
	ctx.Infof("Destroy(%q) called by %v", kName, b)

	email := emailFromBlessingNames(b)
	if email == "" {
		return verror.New(verror.ErrNoAccess, ctx, "unable to determine caller's email address")
	}
	return destroy(ctx, email, kName)
}

// List returns a list of all the instances owned by the caller.
func (i *allocatorImpl) List(ctx *context.T, call rpc.ServerCall) ([]allocator.Instance, error) {
	b, _ := security.RemoteBlessingNames(ctx, call.Security())
	ctx.Infof("List() called by %v", b)

	email := emailFromBlessingNames(b)
	if email == "" {
		return nil, verror.New(verror.ErrNoAccess, ctx, "unable to determine caller's email address")
	}
	return serverInstances(ctx, email)
}

func create(ctx *context.T, email string, baseBlessings security.Blessings, baseBlessingNames []string) (string, error) {
	// Enforce a limit on the number of instances. These tests are a little
	// bit racy. It's possible that multiple calls to create() will run
	// concurrently and that we'll end up with too many instances.
	if n, err := serverInstances(ctx, email); err != nil {
		return "", err
	} else if len(n) >= maxInstancesPerUserFlag {
		return "", verror.New(errLimitExceeded, ctx, maxInstancesPerUserFlag)
	}

	if n, err := serverInstances(ctx, ""); err != nil {
		return "", err
	} else if len(n) >= maxInstancesFlag {
		return "", verror.New(errGlobalLimitExceeded, ctx, maxInstancesFlag)
	}

	kName, err := newKubeName()
	if err != nil {
		return "", err
	}
	mName := mountNameFromKubeName(ctx, kName)

	cfg, cleanup, err := createDeploymentConfig(ctx, email, kName, mName, baseBlessingNames)
	defer cleanup()
	if err != nil {
		return "", err
	}

	vomBlessings, err := vom.Encode(baseBlessings)
	if err != nil {
		return "", err
	}

	if err := createPersistentDisk(ctx, kName); err != nil {
		return "", err
	}

	if _, err := vkube(
		"start", "-f", cfg,
		"--base-blessings", base64.URLEncoding.EncodeToString(vomBlessings),
		"--wait",
		kName,
	); err != nil {
		ctx.Errorf("Error: %v", err)
		deletePersistentDisk(ctx, kName)
		return "", verror.New(verror.ErrInternal, ctx, err)
	}
	return kName, nil
}

func scale(ctx *context.T, email, kName string, replicas int) error {
	args := []string{"kubectl", "scale", "--timeout=1m", fmt.Sprintf("--replicas=%d", replicas), "deployment", kName}
	if _, err := vkube(args...); err != nil {
		ctx.Errorf("Error: %v", err)
		return verror.New(verror.ErrInternal, ctx, err)
	}
	return nil
}

func suspendImpl(ctx *context.T, email, kName string) error {
	return scale(ctx, email, kName, 0)
}

func suspend(ctx *context.T, email, kName string) error {
	if err := isOwnerOfInstance(ctx, email, kName); err != nil {
		return err
	}
	return suspendImpl(ctx, email, kName)
}

func resumeImpl(ctx *context.T, email, kName string) error {
	return scale(ctx, email, kName, 1)
}

func resume(ctx *context.T, email, kName string) error {
	if err := isOwnerOfInstance(ctx, email, kName); err != nil {
		return err
	}
	return resumeImpl(ctx, email, kName)
}

func resetDisk(ctx *context.T, email, kName string) error {
	instance, err := getInstance(ctx, email, kName)
	if err != nil {
		return err
	}
	if instance.Replicas > 0 {
		if err := suspend(ctx, email, kName); err != nil {
			return err
		}
	}
	if err := deletePersistentDisk(ctx, kName); err != nil {
		return err
	}
	if err := createPersistentDisk(ctx, kName); err != nil {
		return err
	}
	if instance.Replicas > 0 {
		return resume(ctx, email, kName)
	}
	return nil
}

func destroy(ctx *context.T, email, kName string) error {
	if err := isOwnerOfInstance(ctx, email, kName); err != nil {
		return err
	}
	cfg, cleanup, err := createDeploymentConfig(ctx, email, kName, "", nil)
	defer cleanup()
	if err != nil {
		return err
	}

	if _, err := vkube("stop", "-f", cfg); err != nil {
		ctx.Errorf("Error: %v", err)
		return verror.New(verror.ErrInternal, ctx, err)
	}
	return deletePersistentDisk(ctx, kName)
}

func createDeploymentConfig(ctx *context.T, email, deploymentName, mountName string, baseBlessingNames []string) (string, func(), error) {
	cleanup := func() {}
	acl, err := accessList(ctx, email)
	if err != nil {
		return "", cleanup, err
	}
	blessingNames := make([]string, len(baseBlessingNames))
	for i, b := range baseBlessingNames {
		blessingNames[i] = b + security.ChainSeparator + deploymentName
	}
	creatorInfo, err := creatorInfo(ctx, email, mountName, blessingNames)
	if err != nil {
		return "", cleanup, err
	}

	t, err := template.ParseFiles(deploymentTemplateFlag)
	if err != nil {
		return "", cleanup, err
	}
	data := struct {
		AccessList  string
		CreatorInfo string
		MountName   string
		Name        string
		OwnerHash   string
		Version     string
	}{
		AccessList:  acl,
		CreatorInfo: creatorInfo,
		MountName:   mountName,
		Name:        deploymentName,
		OwnerHash:   emailHash(email),
		Version:     serverVersionFlag,
	}

	f, err := ioutil.TempFile("", "allocator-deployment-")
	if err != nil {
		return "", cleanup, err
	}
	defer f.Close()
	cleanup = func() { os.Remove(f.Name()) }

	if err := t.Execute(f, data); err != nil {
		return "", cleanup, err
	}
	return f.Name(), cleanup, nil
}

// accessList returns a double encoded JSON access list that can be used in a
// Deployment template that contains something like:
//   "--v23.permissions.literal={\"Admin\": {{.AccessList}} }"
// The access list include the creator.
func accessList(ctx *context.T, email string) (string, error) {
	var acl access.AccessList
	if globalAdminsFlag != "" {
		for _, admin := range strings.Split(globalAdminsFlag, ",") {
			acl.In = append(acl.In, security.BlessingPattern(admin))
		}
	}
	for _, blessing := range conventions.ParseBlessingNames(blessingNamesFromEmail(email)...) {
		acl.In = append(acl.In, blessing.UserPattern())
	}
	j, err := json.Marshal(acl)
	if err != nil {
		ctx.Errorf("json.Marshal(%#v) failed: %v", acl, err)
		return "", err
	}
	// JSON encode again, because the access list is in a JSON template.
	str := string(j)
	j, err = json.Marshal(str)
	if err != nil {
		ctx.Errorf("json.Marshal(%#v) failed: %v", str, err)
		return "", err
	}
	// Remove the quotes.
	return string(j[1 : len(j)-1]), nil
}

type creatorInfoData struct {
	Email         string   `json:"email"`
	BlessingNames []string `json:"blessingNames"`
	MountName     string   `json:"mountName"`
}

// creatorInfo returns a double encoded JSON access list that can be used as
// annotation in a Deployment template, e.g.
//   "annotations": {
//     "v.io/allocator-creator-info": {{.CreatorInfo}}
//   }
func creatorInfo(ctx *context.T, email, mountName string, blessingNames []string) (string, error) {
	j, err := json.Marshal(creatorInfoData{email, blessingNames, mountName})
	if err != nil {
		ctx.Errorf("json.Marshal() failed: %v", err)
		return "", err
	}
	// JSON encode again, because the annotation is in a JSON template.
	str := string(j)
	j, err = json.Marshal(str)
	if err != nil {
		ctx.Errorf("json.Marshal(%#v) failed: %v", str, err)
		return "", err
	}
	return string(j), nil
}

func decodeCreatorInfo(s string) (data creatorInfoData, err error) {
	err = json.Unmarshal([]byte(s), &data)
	return
}

func emailHash(email string) string {
	h := sha1.Sum([]byte(email))
	return hex.EncodeToString(h[:])
}

func serverInstances(ctx *context.T, email string) ([]allocator.Instance, error) {
	args := []string{"kubectl", "get", "deployments", "-o", "json"}
	if email != "" {
		args = append(args, "-l", "ownerHash="+emailHash(email))
	}

	var out []byte
	var err error
	for retry := 0; retry < 10; retry++ {
		if out, err = vkube(args...); err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		return nil, err
	}

	var list struct {
		Items []struct {
			Metadata struct {
				Name         string            `json:"name"`
				CreationTime time.Time         `json:"creationTimestamp"`
				Annotations  map[string]string `json:"annotations"`
			} `json:"metadata"`
			Spec struct {
				Replicas int32 `json:"replicas"`
				Template struct {
					Metadata struct {
						Labels struct {
							Version string `json:"version"`
						} `json:"labels"`
					} `json:"metadata"`
				} `json:"template"`
			} `json:"spec"`
		} `json:"items"`
	}
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, err
	}
	instances := []allocator.Instance{}
	for _, l := range list.Items {
		if !strings.HasPrefix(l.Metadata.Name, serverNameFlag+"-") {
			continue
		}
		cInfo, err := decodeCreatorInfo(l.Metadata.Annotations["v.io/allocator-creator-info"])
		if err != nil {
			ctx.Errorf("decodeCreatorInfo failed: %v", err)
			continue
		}
		instances = append(instances, allocator.Instance{
			Handle:        l.Metadata.Name,
			MountName:     cInfo.MountName,
			BlessingNames: cInfo.BlessingNames,
			CreationTime:  l.Metadata.CreationTime,
			Replicas:      l.Spec.Replicas,
			Version:       l.Spec.Template.Metadata.Labels.Version,
		})
	}
	return instances, nil
}

func getInstance(ctx *context.T, email, kName string) (allocator.Instance, error) {
	instances, err := serverInstances(ctx, email)
	if err != nil {
		return allocator.Instance{}, err
	}
	for _, i := range instances {
		if i.Handle == kName {
			return i, nil
		}
	}
	return allocator.Instance{}, verror.New(verror.ErrNoExistOrNoAccess, nil)
}

func isOwnerOfInstance(ctx *context.T, email, kName string) error {
	_, err := getInstance(ctx, email, kName)
	return err
}

func createPersistentDisk(ctx *context.T, name string) error {
	if out, err := gcloud("compute", "disks", "create", name, "--size", diskSizeFlag); err != nil {
		ctx.Errorf("disk creation failed: %v: %s", err, string(out))
		return err
	}
	return nil
}

func deletePersistentDisk(ctx *context.T, name string) error {
	var (
		start = time.Now()
		out   []byte
		err   error
	)
	for time.Since(start) < 5*time.Minute {
		if out, err = gcloud("compute", "disks", "delete", name); err == nil {
			return nil
		}
		time.Sleep(time.Second)
	}
	ctx.Errorf("disk deletion failed: %v: %s", err, string(out))
	return err
}

func gcloud(args ...string) ([]byte, error) {
	data, err := ioutil.ReadFile(vkubeCfgFlag)
	if err != nil {
		return nil, err
	}
	var config struct {
		Project string `json:"project"`
		Zone    string `json:"zone"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	args = append(args, "--project", config.Project, "--zone", config.Zone)
	return exec.Command(gcloudBinFlag, args...).CombinedOutput()
}

func vkube(args ...string) (out []byte, err error) {
	args = append(
		[]string{
			"--config=" + vkubeCfgFlag,
			"--kubectl=" + kubectlBinFlag,
			"--no-headers",
		},
		args...,
	)
	out, err = exec.Command(vkubeBinFlag, args...).CombinedOutput()
	if err != nil {
		err = fmt.Errorf("vkube(%v) failed: %v\n%s\n", args, err, string(out))
	}
	return
}

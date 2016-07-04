// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"reflect"
	"strings"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/vom"
	"v.io/x/ref/services/cluster"
)

// localAgentAddress returns the address of the cluster agent to use from within
// the cluster.
func localAgentAddress(config *vkubeConfig) string {
	return fmt.Sprintf("/(%s)@%s.%s:%d",
		config.ClusterAgent.Blessing,
		clusterAgentServiceName,
		config.ClusterAgent.Namespace,
		clusterAgentServicePort,
	)
}

// readResourceConfig reads a Deployment config from a file.
func readResourceConfig(fileName string) (object, error) {
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	var dep object
	if err := dep.importJSON(data); err != nil {
		return nil, err
	}
	if kind := dep.getString("kind"); kind != Deployment {
		return nil, fmt.Errorf("expected kind=%q, got %q", Deployment, kind)
	}
	return dep, nil
}

// addPodAgent takes a Deployment, or Pod object and adds a pod-agent container
// to it. The existing containers are updated to use the pod agent.
func addPodAgent(ctx *context.T, config *vkubeConfig, obj object, secretName, rootBlessings string) error {
	var base string
	switch kind := obj.getString("kind"); kind {
	case Deployment:
		base = "spec.template."
	case Pod:
		base = ""
	default:
		return fmt.Errorf("expected kind=%q, or %q, got %q", Deployment, Pod, kind)
	}

	// Add the volumes used by the pod agent container.
	if err := obj.append(base+"spec.volumes",
		object{"name": "agent-logs", "emptyDir": object{}},
		object{"name": "agent-secret", "secret": object{"secretName": secretName}},
		object{"name": "agent-socket", "emptyDir": object{}},
	); err != nil {
		return err
	}

	// Update the existing containers to talk to the pod agent.
	containers := obj.getObjectArray(base + "spec.containers")
	for _, c := range containers {
		if err := c.append("env", object{"name": "V23_AGENT_PATH", "value": "/agent/socket/agent.sock"}); err != nil {
			return err
		}
		if err := c.append("volumeMounts", object{"name": "agent-socket", "mountPath": "/agent/socket", "readOnly": true}); err != nil {
			return err
		}
	}

	// Add the pod agent container.
	containers = append(containers, object{
		"name":            "pod-agent",
		"image":           config.PodAgent.Image,
		"imagePullPolicy": "Always",
		"env": []object{
			object{"name": "ROOT_BLESSINGS", "value": rootBlessings},
		},
		"args": []string{
			"pod_agentd",
			"--agent=" + localAgentAddress(config),
			"--root-blessings=$(ROOT_BLESSINGS)",
			"--secret-key-file=/agent/secret/secret",
			"--socket-path=/agent/socket/agent.sock",
			"--log_dir=/logs",
		},
		"livenessProbe": object{
			"exec": object{"command": []string{
				"env", "V23_AGENT_PATH=/agent/socket/agent.sock", "principal", "dump",
			}},
			"initialDelaySeconds": 5,
			"timeoutSeconds":      1,
		},
		"volumeMounts": []object{
			object{"name": "agent-logs", "mountPath": "/logs"},
			object{"name": "agent-secret", "mountPath": "/agent/secret", "readOnly": true},
			object{"name": "agent-socket", "mountPath": "/agent/socket"},
		},
	})
	return obj.set(base+"spec.containers", containers)
}

// createDeployment takes a Deployment object, adds a pod-agent, and then
// creates it on kubernetes.
func createDeployment(ctx *context.T, config *vkubeConfig, rc object, secretName string) error {
	if err := addPodAgent(ctx, config, rc, secretName, rootBlessings(ctx)); err != nil {
		return err
	}
	if out, err := kubectlCreate(rc, "--record"); err != nil {
		return fmt.Errorf("failed to create deployment: %v\n%s\n", err, string(out))
	}
	return nil
}

// updateDeployment takes a Deployment object, adds a pod-agent (if needed),
// and then applies the update.
func updateDeployment(ctx *context.T, config *vkubeConfig, rc object, secretName, rootBlessings string, stdout, stderr io.Writer) error {
	name := rc.getString("metadata.name")
	namespace := rc.getString("metadata.namespace")

	if secretName != "" {
		if err := addPodAgent(ctx, config, rc, secretName, rootBlessings); err != nil {
			return err
		}
	}
	json, err := rc.json()
	if err != nil {
		return err
	}
	cmd := exec.Command(flagKubectlBin, "replace", "--record", "-f", "-", "--namespace="+namespace)
	cmd.Stdin = bytes.NewBuffer(json)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update deployment %q: %v\n", name, err)
	}
	return nil
}

// watchDeploymentRollout watches the given deployment and makes sure that it
// succeeds. If the rollout gets stuck, it gets rolled back.
func watchDeploymentRollout(name, namespace string, timeout time.Duration, stdout io.Writer) error {
	fmt.Fprintf(stdout, "Rollout progress of %s:\n", name)
	type progress struct {
		obsGeneration, current, updated, available int
	}
	var last progress
	changeTime := time.Now()
	for time.Since(changeTime) < timeout {
		data, err := kubectl("--namespace="+namespace, "get", "deployment", name, "-o", "json")
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		var deployment object
		if err := deployment.importJSON(data); err != nil {
			return fmt.Errorf("failed to parse kubectl output: %v", err)
		}
		// DeploymentSpec is defined at
		// http://kubernetes.io/docs/api-reference/extensions/v1beta1/definitions/#_v1beta1_deploymentspec
		desired := deployment.getInt("spec.replicas", 1)
		generation := deployment.getInt("metadata.generation", -1)

		now := progress{
			// DeploymentStatus is defined at
			// http://kubernetes.io/docs/api-reference/extensions/v1beta1/definitions/#_v1beta1_deploymentstatus
			deployment.getInt("status.observedGeneration", -1),
			deployment.getInt("status.replicas", 0),
			deployment.getInt("status.updatedReplicas", 0),
			deployment.getInt("status.availableReplicas", 0),
		}
		if now != last {
			changeTime, last = time.Now(), now
			if now.obsGeneration < 0 || generation < 0 || now.obsGeneration < generation {
				fmt.Fprintf(stdout, "Not started (Gen: %d vs %d)\n", now.obsGeneration, generation)
			} else {
				fmt.Fprintf(stdout, "Desired: %d Current: %d Up-to-date: %d Available: %d\n", desired, now.current, now.updated, now.available)
			}
		}
		// http://kubernetes.io/docs/user-guide/deployments/#the-status-of-a-deployment
		if now.obsGeneration >= generation && desired == now.current && desired == now.updated && desired == now.available {
			return nil
		}
		time.Sleep(time.Second)
	}
	fmt.Fprintf(stdout, "Something went wrong. Rolling back.\n")
	if _, err := kubectl("--namespace="+namespace, "rollout", "undo", "deployment", name); err != nil {
		return err
	}
	return errors.New("deployment rollout failed, rolled back.")
}

// createNamespaceIfNotExist creates a Namespace object if it doesn't already exist.
func createNamespaceIfNotExist(name string) error {
	if _, err := kubectl("get", "namespace", name); err == nil {
		return nil
	}
	if out, err := kubectlCreate(object{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata": object{
			"name": name,
		},
	}); err != nil {
		return fmt.Errorf("failed to create Namespace %q: %v: %s", name, err, out)
	}
	return nil
}

// findPodAttributes finds the name of the Secret object and root blessings
// associated the given Deployment.
func findPodAttributes(name, namespace string) (string, string, error) {
	var (
		data []byte
		err  error
	)
	if data, err = kubectl("--namespace="+namespace, "get", "deployment", name, "-o", "json"); err != nil {
		return "", "", fmt.Errorf("failed to get deployment %q: %v\n%s\n", name, err, string(data))
	}
	var dep object
	if err := dep.importJSON(data); err != nil {
		return "", "", fmt.Errorf("failed to parse kubectl output: %v", err)
	}

	// Find secret.
	var secret string
	for _, v := range dep.getObjectArray("spec.template.spec.volumes") {
		if v.getString("name") == "agent-secret" {
			secret = v.getString("secret.secretName")
			break
		}
	}

	// Find root blessings.
	var root string
L:
	for _, c := range dep.getObjectArray("spec.template.spec.containers") {
		if c.getString("name") == "pod-agent" {
			for _, e := range c.getObjectArray("env") {
				if e.getString("name") == "ROOT_BLESSINGS" {
					root = e.getString("value")
					break L
				}
			}
		}
	}
	return secret, root, nil
}

// kubectlCreate runs 'kubectl create -f' on the given object and returns the
// output.
func kubectlCreate(o object, extraArgs ...string) ([]byte, error) {
	json, err := o.json()
	if err != nil {
		return nil, err
	}
	args := append([]string{"create", "-f", "-"}, extraArgs...)
	cmd := exec.Command(flagKubectlBin, args...)
	cmd.Stdin = bytes.NewBuffer(json)
	return cmd.CombinedOutput()
}

// kubectl runs the 'kubectl' command with the given arguments and returns the
// output.
func kubectl(args ...string) ([]byte, error) {
	return exec.Command(flagKubectlBin, args...).CombinedOutput()
}

// gcloud runs the 'gcloud' command with the given arguments and returns the
// output.
func gcloud(args ...string) ([]byte, error) {
	return exec.Command(flagGcloudBin, args...).Output()
}

// rootBlessings returns the root blessings for the current principal.
func rootBlessings(ctx *context.T) string {
	p := v23.GetPrincipal(ctx)
	b, _ := p.BlessingStore().Default()
	b64 := []string{}
	for _, root := range security.RootBlessings(b) {
		data, err := vom.Encode(root)
		if err != nil {
			ctx.Fatalf("vom.Encode failed: %v", err)
		}
		// We use URLEncoding to be compatible with the principal
		// command.
		b64 = append(b64, base64.URLEncoding.EncodeToString(data))
	}
	return strings.Join(b64, ",")
}

// upgradeCluster upgrades the cluster to the latest version.
func upgradeCluster(ctx *context.T, config *vkubeConfig, stdin io.Reader, stdout, stderr io.Writer) error {
	data, err := gcloud("--format=json", "container", "get-server-config", "--project", config.Project, "--zone", config.Zone)
	if err != nil {
		return fmt.Errorf("failed to get server config: %v\n%s\n", err, string(data))
	}
	var serverConfig object
	if err := serverConfig.importJSON(data); err != nil {
		return fmt.Errorf("failed to parse gcloud output: %v", err)
	}

	if data, err = gcloud("--format=json", "container", "clusters", "describe", config.Cluster, "--project", config.Project, "--zone", config.Zone); err != nil {
		return fmt.Errorf("failed to get cluster details for %q: %v\n%s\n", config.Cluster, err, string(data))
	}
	var clusterDetails object
	if err := clusterDetails.importJSON(data); err != nil {
		return fmt.Errorf("failed to parse gcloud output: %v", err)
	}
	defaultVersion := serverConfig.getString("defaultClusterVersion")
	masterVersion := clusterDetails.getString("currentMasterVersion")
	nodeVersion := clusterDetails.getString("currentNodeVersion")

	fmt.Fprintf(stdout, "default version: %s\n", defaultVersion)
	fmt.Fprintf(stdout, "master version: %s\n", masterVersion)
	fmt.Fprintf(stdout, "node version: %s\n\n", nodeVersion)

	var upgradeMaster, upgradeNodes bool
	switch {
	case masterVersion != defaultVersion:
		upgradeMaster = true
		upgradeNodes = true
	case nodeVersion != defaultVersion:
		upgradeNodes = true
	default:
		fmt.Fprintf(stdout, "### Nothing to do\n")
		return nil
	}

	if flagPrompt {
		fmt.Fprintf(stdout, "Upgrade the cluster to %q? This can result in several minutes of downtime [y/N]? ", defaultVersion)
		if line, _ := bufio.NewReader(stdin).ReadString('\n'); strings.ToUpper(strings.TrimSpace(line)) != "Y" {
			return errors.New("aborted")
		}
	}

	if upgradeMaster {
		fmt.Fprintf(stdout, "### Upgrading master\n\n")
		cmd := exec.Command(flagGcloudBin, "--quiet", "container", "clusters", "upgrade", config.Cluster, "--project", config.Project, "--zone", config.Zone, "--master")
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	if upgradeNodes {
		fmt.Fprintf(stdout, "\n### Upgrading nodes\n\n")
		cmd := exec.Command(flagGcloudBin, "--quiet", "container", "clusters", "upgrade", config.Cluster, "--project", config.Project, "--zone", config.Zone)
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}

type secretManager struct {
	ctx       *context.T
	namespace string
	agentAddr string
}

func newSecretManager(ctx *context.T, config *vkubeConfig, namespace string) (*secretManager, error) {
	agentAddr, err := findClusterAgent(config, true)
	if err != nil {
		return nil, err
	}
	return &secretManager{ctx, namespace, agentAddr}, nil
}

func (sm *secretManager) create(extension string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	secretName := fmt.Sprintf("secret-%s", hex.EncodeToString(b))

	var baseBlessings security.Blessings
	if flagBaseBlessings == "" {
		baseBlessings, _ = v23.GetPrincipal(sm.ctx).BlessingStore().Default()
	} else {
		b64, err := base64.URLEncoding.DecodeString(flagBaseBlessings)
		if err != nil {
			return "", err
		}
		if err := vom.Decode(b64, &baseBlessings); err != nil {
			return "", err
		}
		if !reflect.DeepEqual(baseBlessings.PublicKey(), v23.GetPrincipal(sm.ctx).PublicKey()) {
			return "", fmt.Errorf("--base-blessings must match the principal's public key")
		}
	}

	secret, err := cluster.ClusterAgentAdminClient(sm.agentAddr).NewSecret(sm.ctx, &granter{blessings: baseBlessings, extension: extension})
	if err != nil {
		return "", err
	}
	if out, err := kubectlCreate(object{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": object{
			"name":      secretName,
			"namespace": sm.namespace,
		},
		"type": "Opaque",
		"data": object{
			"secret": base64.StdEncoding.EncodeToString([]byte(secret)),
		},
	}); err != nil {
		return "", fmt.Errorf("failed to create secret %q: %v\n%s\n", secretName, err, string(out))
	}

	return secretName, nil
}

func (sm *secretManager) delete(name string) error {
	data, err := kubectl("--namespace="+sm.namespace, "get", "secret", name, "-o", "json")
	if err != nil {
		return err
	}
	var secret object
	if err := secret.importJSON(data); err != nil {
		return fmt.Errorf("failed to parse kubectl output: %v", err)
	}
	key, err := base64.StdEncoding.DecodeString(secret.getString("data.secret"))
	if err != nil || len(key) == 0 {
		return fmt.Errorf("failed to decode base64-encoded secret: %v", err)
	}
	if err := cluster.ClusterAgentAdminClient(sm.agentAddr).ForgetSecret(sm.ctx, string(key)); err != nil {
		return fmt.Errorf("ForgetSecret failed: %v", err)
	}
	if _, err := kubectl("--namespace="+sm.namespace, "delete", "secret", name); err != nil {
		return fmt.Errorf("failed to delete secret object: %v", err)
	}
	return nil
}

type granter struct {
	rpc.CallOpt
	blessings security.Blessings
	extension string
}

func (g *granter) Grant(ctx *context.T, call security.Call) (security.Blessings, error) {
	return call.LocalPrincipal().Bless(call.RemoteBlessings().PublicKey(), g.blessings, g.extension, security.UnconstrainedUse())
}

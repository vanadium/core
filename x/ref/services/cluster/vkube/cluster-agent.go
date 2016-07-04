// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/services/device"
)

const (
	clusterAgentServiceName     = "cluster-agent"
	clusterAgentServicePort     = 8193
	clusterAgentApplicationName = "cluster-agentd"
)

func makeClusterAgentObject(config *vkubeConfig, rootBlessings string) object {
	version := "latest"
	if p := strings.Split(config.ClusterAgent.Image, ":"); len(p) == 2 {
		version = p[1]
	}
	ca := object{
		"apiVersion": "extensions/v1beta1",
		"kind":       "Deployment",
		"metadata": object{
			"name": clusterAgentApplicationName,
			"labels": object{
				"application": clusterAgentApplicationName,
			},
			"namespace": config.ClusterAgent.Namespace,
		},
		"spec": object{
			"replicas": 1,
			"selector": object{
				"matchLabels": object{
					"application": clusterAgentApplicationName,
				},
			},
			"minReadySeconds":      60,
			"revisionHistoryLimit": 10,
			"strategy": object{
				"type": "Recreate",
			},
			"template": object{
				"metadata": object{
					"labels": object{
						"application": clusterAgentApplicationName,
						"version":     version,
					},
				},
				"spec": object{
					"containers": []object{
						object{
							"name":            "cluster-agentd",
							"image":           config.ClusterAgent.Image,
							"imagePullPolicy": "Always",
							"ports": []object{
								object{
									"containerPort": clusterAgentServicePort,
								},
							},
							"livenessProbe": object{
								"exec": object{
									"command": []string{
										"vrpc", "identify", fmt.Sprintf("/localhost:%d", clusterAgentServicePort),
									},
								},
								"initialDelaySeconds": 5,
								"timeoutSeconds":      1,
							},
							"readinessProbe": object{
								"exec": object{
									"command": []string{
										"vrpc", "identify", fmt.Sprintf("/localhost:%d", clusterAgentServicePort),
									},
								},
								"initialDelaySeconds": 5,
								"timeoutSeconds":      1,
							},
							"resources": object{
								"limits": object{
									"cpu":    config.ClusterAgent.CPU,
									"memory": config.ClusterAgent.Memory,
								},
							},
							"volumeMounts": []object{
								object{
									"name":      "data",
									"mountPath": "/data",
								},
								object{
									"name":      "logs",
									"mountPath": "/logs",
								},
							},
							"env": []object{
								object{
									"name":  "ROOT_BLESSINGS",
									"value": rootBlessings,
								},
								object{
									"name":  "CLAIMER",
									"value": clusterAgentClaimer(config),
								},
								object{
									"name":  "ADMIN",
									"value": config.ClusterAgent.Admin,
								},
								object{
									"name":  "DATADIR",
									"value": "/data",
								},
								object{
									"name":  "LOGDIR",
									"value": "/logs",
								},
							},
						},
					},
					"volumes": []interface{}{
						object{
							"name":     "logs",
							"emptyDir": object{},
						},
					},
				},
			},
		},
	}
	switch {
	case config.ClusterAgent.PersistentDisk != "":
		ca.append("spec.template.spec.volumes", object{
			"name": "data",
			"gcePersistentDisk": object{
				"pdName": config.ClusterAgent.PersistentDisk,
				"fsType": "ext4",
			},
		})
	case config.ClusterAgent.PersistentVolumeClaim != "":
		ca.append("spec.template.spec.volumes", object{
			"name": "data",
			"persistentVolumeClaim": object{
				"claimName": config.ClusterAgent.PersistentVolumeClaim,
			},
		})
	default:
		ca.append("spec.template.spec.volumes", object{
			"name":     "data",
			"emptyDir": object{},
		})
	}

	return ca
}

// createClusterAgent creates a Deployment and a Service to run the cluster
// agent.
func createClusterAgent(ctx *context.T, config *vkubeConfig) error {
	if err := createNamespaceIfNotExist(config.ClusterAgent.Namespace); err != nil {
		return err
	}
	ca := makeClusterAgentObject(config, rootBlessings(ctx))
	if out, err := kubectlCreate(ca, "--record"); err != nil {
		return fmt.Errorf("failed to create deployment: %v\n%s\n", err, string(out))
	}

	svc := object{
		"apiVersion": "v1",
		"kind":       "Service",
		"metadata": object{
			"name":      clusterAgentServiceName,
			"namespace": config.ClusterAgent.Namespace,
		},
		"spec": object{
			"ports": []object{
				object{
					"port":       clusterAgentServicePort,
					"targetPort": clusterAgentServicePort,
				},
			},
			"selector": object{
				"application": clusterAgentApplicationName,
			},
			"type": "LoadBalancer",
		},
	}
	if config.ClusterAgent.InternalOnly {
		svc.set("spec.type", "ClusterIP")
	} else if config.ClusterAgent.ExternalIP != "" {
		svc.set("spec.loadBalancerIP", config.ClusterAgent.ExternalIP)
	}
	if out, err := kubectlCreate(svc); err != nil {
		return fmt.Errorf("failed to create service: %v\n%s\n", err, string(out))
	}
	return nil
}

// stopClusterAgent stops the cluster agent Deployment and deletes its Service.
func stopClusterAgent(config *vkubeConfig) error {
	if out, err := kubectl("--namespace="+config.ClusterAgent.Namespace, "delete", "deployment", clusterAgentApplicationName); err != nil {
		return fmt.Errorf("failed to stop %s: %v: %s", clusterAgentApplicationName, err, out)
	}
	if out, err := kubectl("--namespace="+config.ClusterAgent.Namespace, "delete", "service", clusterAgentServiceName); err != nil {
		return fmt.Errorf("failed to delete %s: %v: %s", clusterAgentServiceName, err, out)
	}
	return nil
}

// updateClusterAgent updates the Deployment of an existing cluster agent.
func updateClusterAgent(config *vkubeConfig, stdout, stderr io.Writer) error {
	data, err := kubectl("--namespace="+config.ClusterAgent.Namespace, "get", "deployment", clusterAgentApplicationName, "-o", "json")
	if err != nil {
		return fmt.Errorf("failed to get deployment %q: %v\n%s\n", clusterAgentApplicationName, err, string(data))
	}
	var deployment object
	if err := deployment.importJSON(data); err != nil {
		return fmt.Errorf("failed to parse kubectl output: %v", err)
	}

	// Find root blessings.
	containers := deployment.getObjectArray("spec.template.spec.containers")
	if len(containers) != 1 {
		return fmt.Errorf("unexpected number of containers in cluster agent: %d", len(containers))
	}
	var root string
	for _, e := range containers[0].getObjectArray("env") {
		if e.getString("name") == "ROOT_BLESSINGS" {
			root = e.getString("value")
			break
		}
	}

	json, err := makeClusterAgentObject(config, root).json()
	if err != nil {
		return err
	}
	cmd := exec.Command(flagKubectlBin, "replace", "--record", "-f", "-", "--namespace="+config.ClusterAgent.Namespace)
	cmd.Stdin = bytes.NewBuffer(json)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update deployment %q: %v\n", clusterAgentApplicationName, err)
	}
	return nil
}

// clusterAgentClaimer returns the blessing name of the claimer of the cluster
// agent.
func clusterAgentClaimer(config *vkubeConfig) string {
	p := strings.Split(config.ClusterAgent.Blessing, security.ChainSeparator)
	return strings.Join(p[:len(p)-1], security.ChainSeparator)
}

// findClusterAgent returns the external address of the cluster agent.
func findClusterAgent(config *vkubeConfig, includeBlessings bool) (string, error) {
	out, err := kubectl("--namespace="+config.ClusterAgent.Namespace, "get", "service", clusterAgentServiceName, "-o", "json")
	if err != nil {
		return "", fmt.Errorf("failed to get info of %s: %v: %s", clusterAgentServiceName, err, out)
	}
	var svc object
	if err := svc.importJSON(out); err != nil {
		return "", fmt.Errorf("failed to parse kubectl output: %v", err)
	}
	ports := svc.getObjectArray("spec.ports")
	if len(ports) == 0 {
		return "", fmt.Errorf("service %q has no ports", clusterAgentServiceName)
	}
	port := ports[0].getInt("port", -1)
	if port < 0 {
		return "", fmt.Errorf("service %q has no valid port: %v", clusterAgentServiceName, port)
	}
	var ip string
	if config.ClusterAgent.InternalOnly {
		ip = svc.getString("spec.clusterIP")
	} else {
		ingress := svc.getObjectArray("status.loadBalancer.ingress")
		if len(ingress) == 0 {
			return "", fmt.Errorf("service %q has no loadbalancer ingress", clusterAgentServiceName)
		}
		ip = ingress[0].getString("ip")
	}
	if ip == "" {
		return "", fmt.Errorf("service %q loadbalancer has no valid ip", clusterAgentServiceName)
	}
	if includeBlessings {
		return fmt.Sprintf("/(%s)@%s:%d", config.ClusterAgent.Blessing, ip, port), nil
	}
	return fmt.Sprintf("/%s:%d", ip, port), nil
}

// claimClusterAgent claims the cluster agent with the given blessing extension.
func claimClusterAgent(ctx *context.T, config *vkubeConfig, extension string) error {
	addr, err := findClusterAgent(config, false)
	if err != nil {
		return err
	}
	baseBlessings, _ := v23.GetPrincipal(ctx).BlessingStore().Default()
	if err := device.ClaimableClient(addr).Claim(ctx, "", &granter{blessings: baseBlessings, extension: extension}); err != nil {
		return err
	}
	return nil
}

// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"text/template"
	"time"

	"v.io/x/lib/textutil"
	"v.io/x/ref/test/testutil"
	"v.io/x/ref/test/v23test"
)

var (
	flagProject        = flag.String("project", "", "The name of the GCE project to use.")
	flagZone           = flag.String("zone", "", "The name of the GCE zone to use.")
	flagCluster        = flag.String("cluster", "", "The name of the kubernetes cluster to use.")
	flagGetCredentials = flag.Bool("get-credentials", true, "This flag is passed to vkube.")
	flagDockerRegistry = flag.String("docker-registry", "", "The docker registry to use. If empty, a temporary bucket in <project> is used.")
	flagRebuildProb    = flag.Float64("rebuild-probability", 0.05, "A number between 0 and 1 to control how often the docker images are rebuilt. A value of 0 means never. A value of 1 means always. This flag is only meaningful when --docker-registry is set.")
	flagTestNamespace  = flag.String("test-namespace", "", "The name of the kubernetes namespace to use for this test instance. If empty, a temporary namespace is used.")
)

// TestV23Vkube is an end-to-end test for the vkube command. It operates on a
// pre-existing kubernetes cluster running on GCE.
// This test can easily exceed the default test timeout of 10m. It is
// recommended to use -test.timeout=20m.
func TestV23Vkube(t *testing.T) {
	if *flagProject == "" || (*flagGetCredentials && (*flagZone == "" || *flagCluster == "")) {
		t.Skip("--project, --zone, or --cluster not specified")
	}
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	workdir := sh.MakeTempDir()

	id := fmt.Sprintf("vkube-test-%s-%08x", time.Now().UTC().Format("20060102-150405"), rand.Uint32())

	namespace := *flagTestNamespace
	if namespace == "" {
		namespace = id
	}

	dockerRegistry := *flagDockerRegistry
	if dockerRegistry == "" {
		dockerRegistry = "b.gcr.io/" + id
	}

	vkubeCfgPath := filepath.Join(workdir, "vkube.cfg")
	if err := createVkubeConfig(vkubeCfgPath, namespace, dockerRegistry); err != nil {
		t.Fatal(err)
	}

	creds := sh.ForkCredentials("alice")

	vkubeBin := v23test.BuildGoPkg(sh, "v.io/x/ref/services/cluster/vkube")
	vshBin := v23test.BuildGoPkg(sh, "v.io/x/ref/examples/tunnel/vsh")

	var (
		cmd = func(name string, expectSuccess bool, baseArgs ...string) func(args ...string) string {
			return func(args ...string) string {
				args = append(baseArgs, args...)
				fmt.Printf("Running: %s %s\nExpect success: %v\n", name, strings.Join(args, " "), expectSuccess)
				// Note, creds do not affect non-Vanadium commands.
				c := sh.Cmd(name, args...).WithCredentials(creds)
				c.ExitErrorIsOk = true
				plw := textutil.PrefixLineWriter(os.Stdout, filepath.Base(name)+"> ")
				c.AddStdoutWriter(plw)
				c.AddStderrWriter(plw)
				output := c.CombinedOutput()
				plw.Flush()
				if expectSuccess && c.Err != nil {
					t.Error(testutil.FormatLogLine(2, "Unexpected failure: %s %s :%v", name, strings.Join(args, " "), c.Err))
				} else if !expectSuccess && c.Err == nil {
					t.Error(testutil.FormatLogLine(2, "Unexpected success %s: %s", name, strings.Join(args, " ")))
				}
				return output
			}
		}
		timedCmd = func(name string, d time.Duration, baseArgs ...string) func(args ...string) (string, error) {
			return func(args ...string) (string, error) {
				args = append(baseArgs, args...)
				fmt.Printf("Running: %s %s\n", name, strings.Join(args, " "))
				// Note, creds do not affect non-Vanadium commands.
				c := sh.Cmd(name, args...).WithCredentials(creds)
				c.ExitErrorIsOk = true
				plw := textutil.PrefixLineWriter(os.Stdout, fmt.Sprintf("%s(%s)> ", filepath.Base(name), d))
				// gosh.Cmd isn't thread-safe. We can't call c.CombinedOutput() in this goroutine and
				// c.Pid() in another one. Instead, we must use c.AddStdoutWriter(), c.AddStderrWriter(),
				// c.Start(), c.Pid(), and c.Wait(), all in the same goroutine. Then, use syscall.Kill()
				// in the other goroutine.
				var output bytes.Buffer
				c.AddStdoutWriter(&output)
				c.AddStderrWriter(&output)
				c.AddStdoutWriter(plw)
				c.AddStderrWriter(plw)
				c.Start()
				exit := make(chan struct{})
				go func(pid int) {
					select {
					case <-exit:
					case <-time.After(d):
						syscall.Kill(-pid, syscall.SIGINT)
						time.Sleep(time.Second)
						syscall.Kill(-pid, syscall.SIGKILL)
					}
				}(c.Pid())
				c.Wait()
				close(exit)
				plw.Flush()
				return output.String(), c.Err
			}
		}
		retry = func(name string, baseArgs ...string) func(args ...string) string {
			c := timedCmd(name, 5*time.Minute, baseArgs...)
			return func(args ...string) (out string) {
				var err error
				for attempt := 0; attempt < 5; attempt++ {
					if out, err = c(args...); err == nil {
						return out
					}
					time.Sleep(10 * time.Second)
				}
				t.Errorf("Unexpected failure: %s %s: %v", name, strings.Join(args, " "), err)
				return
			}
		}
		gsutil      = retry("gsutil")
		gcloud      = retry("gcloud", "--project="+*flagProject)
		docker      = retry("docker")
		getCreds    = fmt.Sprintf("--get-credentials=%v", *flagGetCredentials)
		vkubeOK     = cmd(vkubeBin, true, "--config="+vkubeCfgPath, getCreds, "--no-headers")
		vkubeFail   = cmd(vkubeBin, false, "--config="+vkubeCfgPath, getCreds, "--no-headers")
		kubectlOK   = cmd(vkubeBin, true, "--config="+vkubeCfgPath, getCreds, "--no-headers", "kubectl", "--", "--namespace="+namespace)
		kubectlFail = cmd(vkubeBin, false, "--config="+vkubeCfgPath, getCreds, "--no-headers", "kubectl", "--", "--namespace="+namespace)
		kubectl5s   = timedCmd(vkubeBin, 5*time.Second, "--config="+vkubeCfgPath, getCreds, "--no-headers", "kubectl", "--", "--namespace="+namespace)
		vshOK       = cmd(vshBin, true)
	)

	if out := kubectlOK("cluster-info"); strings.Contains(out, "ERROR:") {
		// Exit early if we don't have valid credentials.
		t.Fatalf("Failed to get cluster information: %v", out)
	}

	if *flagDockerRegistry == "" {
		// Use a temporary bucket as docker registry.
		gsutil("mb", "-p", *flagProject, "gs://"+id)
		defer func() {
			gsutil("-m", "rm", "-r", "gs://"+id)
		}()
	}

	if *flagTestNamespace == "" {
		kubectlOK("create", "namespace", namespace)
		defer func() {
			kubectlOK("delete", "namespace", namespace)
		}()
	}

	// Create app's docker image and configs.
	appImage := dockerRegistry + "/tunneld:latest"
	badImage := dockerRegistry + "/not-found"

	dockerDir, err := setupDockerDirectory(workdir)
	if err != nil {
		t.Fatal(err)
	}
	conf := make(map[string]string)
	for _, c := range []struct{ name, version, kind string }{
		{"app1", "1", "deploy"},
		{"app2", "2", "deploy"},
		{"app-bad", "bad", "deploy-bad"},
		{"bb1", "1", "busybox"},
		{"bb2", "2", "busybox"},
	} {
		file := filepath.Join(workdir, c.name+".json")
		conf[c.name] = file
		var err error
		switch c.kind {
		case "deploy":
			err = createAppDeploymentConfig(file, namespace, appImage, c.version)
		case "deploy-bad":
			err = createAppDeploymentConfig(file, namespace, badImage, c.version)
		case "busybox":
			err = createBusyboxConfig(file, namespace, c.version)
		default:
			err = fmt.Errorf("%s?", c.kind)
		}
		if err != nil {
			t.Fatal(err)
		}
	}

	if *flagDockerRegistry == "" || rand.Float64() < *flagRebuildProb {
		t.Log("Rebuilding docker images")
		docker("build", "-t", appImage, dockerDir)
		gcloud("docker", "push", appImage)

		vkubeOK("build-docker-images", "-v", "-tag=1")
		vkubeOK("build-docker-images", "-v", "-tag=2")

		// Clean up local docker images.
		docker(
			"rmi", "-f",
			dockerRegistry+"/cluster-agent",
			dockerRegistry+"/cluster-agent:1",
			dockerRegistry+"/cluster-agent:2",
			dockerRegistry+"/pod-agent",
			dockerRegistry+"/pod-agent:1",
			dockerRegistry+"/pod-agent:2",
			dockerRegistry+"/tunneld",
		)
	}

	// Run the actual tests.
	vkubeOK("update-config",
		"--cluster-agent-image="+dockerRegistry+"/cluster-agent:1",
		"--pod-agent-image="+dockerRegistry+"/pod-agent:1")
	vkubeOK("start-cluster-agent", "--wait")
	vkubeOK("update-config", "--cluster-agent-image=:2", "--pod-agent-image=:2")
	vkubeOK("update-cluster-agent", "--wait")
	kubectlOK("get", "service", "cluster-agent")
	kubectlOK("get", "deployment", "cluster-agentd")
	kubectlOK("get", "rs,pods", "-Lversion")
	vkubeFail("start-cluster-agent") // Already running
	vkubeOK("claim-cluster-agent")
	vkubeFail("claim-cluster-agent") // Already claimed

	vkubeOK("start", "-f", conf["app1"], "--wait", "my-app")
	kubectlOK("get", "deployment", "tunneld")
	vkubeFail("start", "-f", conf["app1"], "my-app") // Already running

	vkubeOK("update", "-f", conf["app2"], "--wait")
	vkubeOK("update", "-f", conf["app2"], "--wait", "my-app2")
	kubectlOK("describe", "deployment", "tunneld")
	kubectlOK("get", "pod", "--show-labels")

	// Find the pod running tunneld, get the server's addr from its stdout.
	podName := kubectlOK("get", "pod", "-l", "application=tunneld,version=2", "--template={{range .items}}{{.metadata.name}}{{end}}")
	if podName == "" {
		t.Errorf("Failed to get pod name of tunneld")
	} else {
		var addr string
		for addr == "" {
			time.Sleep(100 * time.Millisecond)
			logs, err := kubectl5s("logs", podName, "-c", "tunneld")
			if err != nil {
				t.Logf("kubectl logs failed: %v", err)
				continue
			}
			for _, log := range strings.Split(logs, "\n") {
				if strings.HasPrefix(log, "NAME=") {
					addr = strings.TrimPrefix(log, "NAME=")
					break
				}
			}
		}
		if got, expected := vshOK(addr, "echo", "hello", "world"), "hello world\n"; got != expected {
			t.Errorf("Unexpected output. Got %q, expected %q", got, expected)
		}
	}
	vkubeFail("update", "-f", conf["app-bad"], "--wait", "--wait-timeout=30s")
	if out := kubectlOK("describe", "deployment", "tunneld"); !strings.Contains(out, "DeploymentRollback") {
		t.Error("expected a rollback in the deployment events")
	}

	vkubeOK("stop", "-f", conf["app2"])
	kubectlFail("get", "deployment", "tunneld") // No longer running
	vkubeFail("stop", "-f", conf["app2"])       // No longer running

	// App with no blessings.
	vkubeOK("start", "-f", conf["bb1"], "--noblessings", "--wait")
	vkubeFail("start", "-f", conf["bb1"], "--noblessings") // Already running
	vkubeOK("update", "-f", conf["bb2"], "--wait")
	vkubeOK("stop", "-f", conf["bb2"])
	vkubeFail("stop", "-f", conf["bb2"]) // No longer running

	vkubeOK("stop-cluster-agent")
	kubectlFail("get", "service", "cluster-agent")
	kubectlFail("get", "deployment", "cluster-agentd")
}

func createVkubeConfig(path, namespace, dockerRegistry string) error {
	internalOnly := false
	if !*flagGetCredentials && os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		// The test is running on kubernetes and it is using the same
		// cluster to run the test applications. There is no need to
		// allocate an external IP address for the cluster agent.
		internalOnly = true
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	params := struct {
		Project, Zone, Cluster, Registry, Namespace string
		InternalOnly                                bool
	}{
		*flagProject,
		*flagZone,
		*flagCluster,
		dockerRegistry,
		namespace,
		internalOnly,
	}
	return template.Must(template.New("cfg").Parse(`{
  "project": "{{.Project}}",
  "zone": "{{.Zone}}",
  "cluster": "{{.Cluster}}",
  "clusterAgent": {
    "namespace": "{{.Namespace}}",
    "image": "{{.Registry}}/cluster-agent:xxx",
    "blessing": "root:alice:cluster-agent",
    "admin": "root:alice,root:bob",
    "cpu": "0.1",
    "memory": "100M",
    "internalOnly": {{.InternalOnly}}
  },
  "podAgent": {
    "image": "{{.Registry}}/pod-agent:xxx"
  }
}`)).Execute(f, params)
}

func setupDockerDirectory(workdir string) (string, error) {
	dockerDir := filepath.Join(workdir, "docker")
	if err := os.Mkdir(dockerDir, 0755); err != nil {
		return "", err
	}
	if err := ioutil.WriteFile(
		filepath.Join(dockerDir, "Dockerfile"),
		[]byte("FROM busybox\nCOPY tunneld /usr/local/bin/\n"),
		0644,
	); err != nil {
		return "", err
	}
	if out, err := exec.Command("jiri", "go", "build",
		"-o", filepath.Join(dockerDir, "tunneld"),
		"-ldflags", "-extldflags -static",
		"v.io/x/ref/examples/tunnel/tunneld").CombinedOutput(); err != nil {
		return "", fmt.Errorf("build failed: %v: %s", err, string(out))
	}
	return dockerDir, nil
}

func createAppDeploymentConfig(path, namespace, image, version string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	params := struct{ Namespace, Image, Version string }{namespace, image, version}
	return template.Must(template.New("appcfg").Parse(`{
  "apiVersion": "extensions/v1beta1",
  "kind": "Deployment",
  "metadata": {
    "name": "tunneld",
    "namespace": "{{.Namespace}}",
    "labels": {
      "application": "tunneld"
    }
  },
  "spec": {
    "replicas": 1,
    "selector": {
      "matchLabels": {
        "application": "tunneld"
      }
    },
    "minReadySeconds": 5,
    "template": {
      "metadata": {
        "labels": {
          "application": "tunneld",
          "version": "{{.Version}}"
        }
      },
      "spec": {
        "containers": [
          {
            "name": "tunneld",
            "image": "{{.Image}}",
            "command": [
              "tunneld",
              "--v23.tcp.address=:8193",
              "--v23.permissions.literal={\"Admin\":{\"In\":[\"root:alice\"]}}",
	      "--alsologtostderr=false"
            ],
            "ports": [
              { "containerPort": 8193, "hostPort": 8193 }
            ],
            "readinessProbe": {
              "tcpSocket": { "port": 8193 },
              "initialDelaySeconds": 5,
              "timeoutSeconds": 1
            },
            "resources": {
              "limits": { "cpu": "0.1", "memory": "100M" }
            }
          }
        ]
      }
    }
  }
}`)).Execute(f, params)
}

func createBusyboxConfig(path, namespace, version string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	params := struct{ Namespace, Version string }{namespace, version}
	return template.Must(template.New("appcfg").Parse(`{
  "apiVersion": "extensions/v1beta1",
  "kind": "Deployment",
  "metadata": {
    "name": "busybox",
    "namespace": "{{.Namespace}}",
    "labels": {
      "application": "busybox"
    }
  },
  "spec": {
    "replicas": 1,
    "selector": {
      "matchLabels": {
        "application": "busybox"
      }
    },
    "template": {
      "metadata": {
        "labels": {
          "application": "busybox",
          "version": "{{.Version}}"
        }
      },
      "spec": {
        "containers": [
          {
            "name": "busybox",
            "image": "busybox",
            "command": [ "sleep", "3600" ]
          }
        ]
      }
    }
  }
}`)).Execute(f, params)
}

func TestMain(m *testing.M) {
	v23test.TestMain(m)
}

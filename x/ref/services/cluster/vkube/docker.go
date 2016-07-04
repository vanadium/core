// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	clusterAgentDockerfile = `
FROM busybox
COPY claimable cluster_agentd principal vrpc init.sh /usr/local/bin/
RUN chmod 755 /usr/local/bin/*
CMD ["/usr/local/bin/init.sh"]
`
	clusterAgentInitSh = `#!/bin/sh
export V23_CREDENTIALS_NO_AGENT=true
if [ ! -e "${DATADIR}/perms" ]; then
  # Not claimed
  if [ ! -e "${DATADIR}/creds" ]; then
    /usr/local/bin/principal create --with-passphrase=false "${DATADIR}/creds" cluster-agent
  fi
  /usr/local/bin/claimable \
    --v23.credentials="${DATADIR}/creds" \
    --v23.tcp.address=:8193 \
    --root-blessings="${ROOT_BLESSINGS}" \
    --perms-dir="${DATADIR}/perms" \
    --v23.permissions.literal="{\"Admin\":{\"In\":[\"${CLAIMER}\"]}}" \
    --log_dir="${LOGDIR}" \
    --alsologtostderr=false
fi

mkdir -p "${DATADIR}/blessings"

# Handle multiple admins, separated by commas.
ADMIN=$(echo "${ADMIN}" | sed 's:,:",":g')

exec /usr/local/bin/cluster_agentd \
  --v23.credentials="${DATADIR}/creds" \
  --v23.tcp.address=:8193 \
  --v23.permissions.literal="{\"Admin\":{\"In\":[\"${ADMIN}\"]}}" \
  --log_dir="${LOGDIR}" \
  --root-dir="${DATADIR}/blessings" \
  --alsologtostderr=false
`

	podAgentDockerfile = `
FROM busybox
COPY pod_agentd principal /usr/local/bin/
`
)

type dockerFile struct {
	name    string
	content []byte
}

type dockerCmd struct {
	name string
	args []string
}

func buildDockerImages(config *vkubeConfig, tag string, verbose bool, stdout io.Writer) error {
	if tag == "" {
		tag = time.Now().Format("20060102150405")
	}
	// Cluster agent image.
	imageName := removeTag(config.ClusterAgent.Image)
	imageNameTag := fmt.Sprintf("%s:%s", imageName, tag)

	var out io.Writer
	if verbose {
		out = stdout
	}

	if err := buildDockerImage([]dockerFile{
		{"Dockerfile", []byte(clusterAgentDockerfile)},
		{"init.sh", []byte(clusterAgentInitSh)},
	}, []dockerCmd{
		{"jiri", goBuildArgs("claimable", "v.io/x/ref/services/device/claimable")},
		{"jiri", goBuildArgs("cluster_agentd", "v.io/x/ref/services/cluster/cluster_agentd")},
		{"jiri", goBuildArgs("principal", "v.io/x/ref/cmd/principal")},
		{"jiri", goBuildArgs("vrpc", "v.io/x/ref/cmd/vrpc")},
		{"docker", []string{"build", "-t", imageName, "."}},
		{"docker", []string{"tag", "-f", imageName, imageNameTag}},
		{flagGcloudBin, []string{"--project=" + config.Project, "docker", "push", imageName}},
	}, out); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Pushed %s successfully.\n", imageNameTag)

	// Pod agent image.
	imageName = removeTag(config.PodAgent.Image)
	imageNameTag = fmt.Sprintf("%s:%s", imageName, tag)

	if err := buildDockerImage([]dockerFile{
		{"Dockerfile", []byte(podAgentDockerfile)},
	}, []dockerCmd{
		{"jiri", goBuildArgs("pod_agentd", "v.io/x/ref/services/agent/pod_agentd")},
		{"jiri", goBuildArgs("principal", "v.io/x/ref/cmd/principal")},
		{"docker", []string{"build", "-t", imageName, "."}},
		{"docker", []string{"tag", "-f", imageName, imageNameTag}},
		{flagGcloudBin, []string{"--project=" + config.Project, "docker", "push", imageName}},
	}, out); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Pushed %s successfully.\n", imageNameTag)
	return nil
}

func goBuildArgs(binary, target string) []string {
	return []string{
		"go",
		"build",
		"-o",
		binary,
		"-ldflags", "-extldflags -static",
		target,
	}
}

func removeTag(name string) string {
	if p := strings.Split(name, ":"); len(p) > 0 {
		return p[0]
	}
	return ""
}

func buildDockerImage(files []dockerFile, cmds []dockerCmd, stdout io.Writer) error {
	workDir, err := ioutil.TempDir("", "docker-build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)

	for _, f := range files {
		if stdout != nil {
			fmt.Fprintf(stdout, "#### Writing %q\n", f.name)
		}
		if err := ioutil.WriteFile(filepath.Join(workDir, f.name), f.content, 0600); err != nil {
			return fmt.Errorf("failed to write %q: %v", f.name, err)
		}
	}
	for _, c := range cmds {
		var err error
		for attempt := 1; attempt <= 3; attempt++ {
			if stdout != nil {
				fmt.Fprintf(stdout, "#### Running %s %s [%d/3]\n", c.name, strings.Join(c.args, " "), attempt)
			}
			cmd := exec.Command(c.name, c.args...)
			cmd.Dir = workDir
			cmd.Stdout = stdout
			cmd.Stderr = stdout
			if err = cmd.Run(); err == nil {
				break
			}
			time.Sleep(10 * time.Second)
		}
		if err != nil {
			return fmt.Errorf("%v failed: %v", c, err)
		}
	}
	return nil
}

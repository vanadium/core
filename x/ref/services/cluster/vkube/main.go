// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $JIRI_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go .

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/generic"
)

var (
	flagConfigFile        string
	flagKubectlBin        string
	flagGcloudBin         string
	flagGpg               string
	flagGetCredentials    bool
	flagNoBlessings       bool
	flagBaseBlessings     string
	flagNoHeaders         bool
	flagResourceFile      string
	flagVerbose           bool
	flagTag               string
	flagWait              bool
	flagWaitTimeout       time.Duration
	flagClusterAgentImage string
	flagPodAgentImage     string
	flagSecrets           string
	flagPrompt            bool
)

const (
	Deployment = "Deployment"
	Pod        = "Pod"
)

func main() {
	cmdline.HideGlobalFlagsExcept()

	cmd := &cmdline.Command{
		Name:  "vkube",
		Short: "Manages Vanadium applications on kubernetes",
		Long:  "Manages Vanadium applications on kubernetes",
		Children: []*cmdline.Command{
			cmdStart,
			cmdUpdate,
			cmdStop,
			cmdStartClusterAgent,
			cmdStopClusterAgent,
			cmdUpdateClusterAgent,
			cmdUpdateConfig,
			cmdClaimClusterAgent,
			cmdBuildDockerImages,
			cmdCreateSecrets,
			cmdUpgradeCluster,
			cmdKubectl,
		},
	}
	cmd.Flags.StringVar(&flagConfigFile, "config", "vkube.cfg", "The 'vkube.cfg' file to use.")
	cmd.Flags.StringVar(&flagKubectlBin, "kubectl", "kubectl", "The 'kubectl' binary to use.")
	cmd.Flags.StringVar(&flagGcloudBin, "gcloud", "gcloud", "The 'gcloud' binary to use.")
	cmd.Flags.BoolVar(&flagGetCredentials, "get-credentials", true, "When true, use gcloud to get the cluster credentials. Otherwise, assume kubectl already has the correct credentials, and 'vkube kubectl' is equivalent to 'kubectl'.")
	cmd.Flags.BoolVar(&flagNoHeaders, "no-headers", false, "When true, suppress the 'Project: ... Zone: ... Cluster: ...' headers.")

	cmdStart.Flags.BoolVar(&flagNoBlessings, "noblessings", false, "Do not pass blessings to the application.")
	cmdStart.Flags.StringVar(&flagBaseBlessings, "base-blessings", "", "Base blessings to extend, base64url-vom-encoded. If empty, the default blessings are used.")
	cmdStart.Flags.StringVar(&flagResourceFile, "f", "", "Filename to use to create the kubernetes resource.")
	cmdStart.Flags.BoolVar(&flagWait, "wait", false, "Wait for all the replicas to be ready.")
	cmdStart.Flags.DurationVar(&flagWaitTimeout, "wait-timeout", 5*time.Minute, "How long to wait for the start to make progress.")

	cmdUpdate.Flags.StringVar(&flagResourceFile, "f", "", "Filename to use to update the kubernetes resource.")
	cmdUpdate.Flags.StringVar(&flagBaseBlessings, "base-blessings", "", "Base blessings to extend, base64url-vom-encoded. If empty, the default blessings are used.")
	cmdUpdate.Flags.BoolVar(&flagWait, "wait", false, "Wait for the update to finish.")
	cmdUpdate.Flags.DurationVar(&flagWaitTimeout, "wait-timeout", 5*time.Minute, "How long to wait for the update to make progress.")

	cmdStop.Flags.StringVar(&flagResourceFile, "f", "", "Filename to use to stop the kubernetes resource.")

	cmdStartClusterAgent.Flags.BoolVar(&flagWait, "wait", false, "Wait for the cluster agent to be ready.")
	cmdStartClusterAgent.Flags.DurationVar(&flagWaitTimeout, "wait-timeout", 5*time.Minute, "How long to wait for the cluster agent to be ready.")

	cmdUpdateClusterAgent.Flags.BoolVar(&flagWait, "wait", false, "Wait for the cluster agent to be ready.")
	cmdUpdateClusterAgent.Flags.DurationVar(&flagWaitTimeout, "wait-timeout", 5*time.Minute, "How long to wait for the cluster agent to be ready.")

	cmdUpdateConfig.Flags.StringVar(&flagClusterAgentImage, "cluster-agent-image", "", "The new cluster agent image. If the name starts with ':', only the image tag is updated.")
	cmdUpdateConfig.Flags.StringVar(&flagPodAgentImage, "pod-agent-image", "", "The new pod agent image. If the name starts with ':', only the image tag is updated.")

	cmdBuildDockerImages.Flags.BoolVar(&flagVerbose, "v", false, "When true, the output is more verbose.")
	cmdBuildDockerImages.Flags.StringVar(&flagTag, "tag", "", "The tag to add to the docker images. If empty, the current timestamp is used.")

	cmdCreateSecrets.Flags.StringVar(&flagSecrets, "secrets", "", "The file containing the encrypted secrets.")
	cmdCreateSecrets.Flags.StringVar(&flagGpg, "gpg", "gpg", "The gpg binary to use.")

	cmdUpgradeCluster.Flags.BoolVar(&flagPrompt, "prompt", true, "When true, prompt for confirmation before upgrading the cluster.")

	cmdline.Main(cmd)
}

func kubeCmdRunner(kcmd func(ctx *context.T, env *cmdline.Env, args []string, config *vkubeConfig) error) cmdline.Runner {
	return v23cmd.RunnerFunc(func(ctx *context.T, env *cmdline.Env, args []string) error {
		config, err := readConfig(flagConfigFile)
		if err != nil {
			return err
		}
		if flagGetCredentials {
			if !flagNoHeaders {
				fmt.Fprintf(env.Stderr, "Project: %s Zone: %s Cluster: %s\n\n", config.Project, config.Zone, config.Cluster)
			}
			f, err := ioutil.TempFile("", "kubeconfig-")
			if err != nil {
				return err
			}
			os.Setenv("KUBECONFIG", f.Name())
			defer os.Remove(f.Name())
			f.Close()

			if out, err := exec.Command(flagGcloudBin, "container", "clusters", "get-credentials", config.Cluster, "--project", config.Project, "--zone", config.Zone).CombinedOutput(); err != nil {
				return fmt.Errorf("failed to get credentials for %q: %v: %s", config.Cluster, err, out)
			}
		}
		return kcmd(ctx, env, args, config)
	})
}

var cmdStart = &cmdline.Command{
	Runner:   kubeCmdRunner(runCmdStart),
	Name:     "start",
	Short:    "Starts an application.",
	Long:     "Starts an application.",
	ArgsName: "<extension>",
	ArgsLong: `<extension> The blessing name extension to give to the application.

If --noblessings is set, this argument is not needed.
`,
}

func runCmdStart(ctx *context.T, env *cmdline.Env, args []string, config *vkubeConfig) error {
	if flagNoBlessings && len(args) != 0 {
		return env.UsageErrorf("start: no arguments are expected when --noblessings is set")
	}
	if !flagNoBlessings && len(args) != 1 {
		return env.UsageErrorf("start: expected one argument, got %d", len(args))
	}
	if flagResourceFile == "" {
		return env.UsageErrorf("-f must be specified.")
	}
	dep, err := readResourceConfig(flagResourceFile)
	if err != nil {
		return err
	}
	namespace := dep.getString("metadata.namespace")
	appName := dep.getString("spec.template.metadata.labels.application")

	if flagNoBlessings {
		if out, err := kubectlCreate(dep); err != nil {
			fmt.Fprintln(env.Stderr, string(out))
			return err
		}
	} else {
		sm, err := newSecretManager(ctx, config, namespace)
		if err != nil {
			return err
		}
		secretName, err := sm.create(args[0])
		if err != nil {
			return err
		}
		fmt.Fprintf(env.Stdout, "Created secret %q.\n", secretName)

		if err := createDeployment(ctx, config, dep, secretName); err != nil {
			if err := sm.delete(secretName); err != nil {
				fmt.Fprintf(env.Stderr, "Failed to delete secret %q: %v\n", secretName, err)
			} else {
				fmt.Fprintf(env.Stdout, "Deleted secret %q.\n", secretName)
			}
			return err
		}
	}
	if flagWait {
		if err := watchDeploymentRollout(appName, namespace, flagWaitTimeout, env.Stdout); err != nil {
			return err
		}
	}
	fmt.Fprintln(env.Stdout, "Created deployment successfully.")
	return nil
}

var cmdUpdate = &cmdline.Command{
	Runner:   kubeCmdRunner(runCmdUpdate),
	Name:     "update",
	Short:    "Updates an application.",
	Long:     "Updates an application to a new version with a rolling update.",
	ArgsName: "[<extension>]",
	ArgsLong: "<extension> The new blessing name extension to give to the application. If omitted, the existing blessings are preserved.",
}

func runCmdUpdate(ctx *context.T, env *cmdline.Env, args []string, config *vkubeConfig) error {
	if flagResourceFile == "" {
		return env.UsageErrorf("-f must be specified.")
	}
	dep, err := readResourceConfig(flagResourceFile)
	if err != nil {
		return err
	}
	name := dep.getString("metadata.name")
	namespace := dep.getString("metadata.namespace")

	oldSecretName, rootB, err := findPodAttributes(name, namespace)
	if err != nil {
		return err
	}

	var (
		secretName string
		sm         *secretManager
	)
	switch len(args) {
	case 0: // re-use existing secret
		secretName = oldSecretName
	case 1: // create new secret
		rootB = rootBlessings(ctx)
		sm, err = newSecretManager(ctx, config, namespace)
		if err != nil {
			return err
		}
		if secretName, err = sm.create(args[0]); err != nil {
			return err
		}
		fmt.Fprintf(env.Stdout, "Created new secret %q.\n", secretName)
	default:
		return env.UsageErrorf("update: expected at most one argument, got %d", len(args))
	}

	if err := updateDeployment(ctx, config, dep, secretName, rootB, env.Stdout, env.Stderr); err != nil {
		if sm != nil {
			if err := sm.delete(secretName); err != nil {
				fmt.Fprintf(env.Stderr, "Failed to delete new secret %q: %v\n", secretName, err)
			} else {
				fmt.Fprintf(env.Stdout, "Deleted new secret %q.\n", secretName)
			}
		}
		return err
	}
	if flagWait {
		if err := watchDeploymentRollout(name, namespace, flagWaitTimeout, env.Stdout); err != nil {
			return err
		}
	}
	fmt.Fprintln(env.Stdout, "Updated deployment successfully.")

	if sm != nil && oldSecretName != "" {
		if err := sm.delete(oldSecretName); err != nil {
			fmt.Fprintf(env.Stderr, "Failed to delete old secret %q: %v\n", oldSecretName, err)
		} else {
			fmt.Fprintf(env.Stdout, "Deleted old secret %q.\n", oldSecretName)
		}
	}

	return nil
}

var cmdStop = &cmdline.Command{
	Runner: kubeCmdRunner(runCmdStop),
	Name:   "stop",
	Short:  "Stops an application.",
	Long:   "Stops an application.",
}

func runCmdStop(ctx *context.T, env *cmdline.Env, args []string, config *vkubeConfig) error {
	if flagResourceFile == "" {
		return env.UsageErrorf("-f must be specified.")
	}
	dep, err := readResourceConfig(flagResourceFile)
	if err != nil {
		return err
	}
	name := dep.getString("metadata.name")
	if name == "" {
		return fmt.Errorf("metadata.name must be set")
	}
	namespace := dep.getString("metadata.namespace")
	secretName, _, err := findPodAttributes(name, namespace)
	if err != nil {
		return err
	}
	if out, err := kubectl("--namespace="+namespace, "delete", "deployment", name); err != nil {
		return fmt.Errorf("failed to stop deployment: %v: %s", err, out)
	}
	fmt.Fprintln(env.Stdout, "Stopped deployment.")
	if secretName != "" {
		sm, err := newSecretManager(ctx, config, namespace)
		if err != nil {
			return err
		}
		if err := sm.delete(secretName); err != nil {
			fmt.Fprintf(env.Stderr, "Failed to delete secret %q: %v\n", secretName, err)
			return err
		}
		fmt.Fprintf(env.Stdout, "Deleted secret %q.\n", secretName)
	}
	return nil
}

var cmdStartClusterAgent = &cmdline.Command{
	Runner: kubeCmdRunner(runCmdStartClusterAgent),
	Name:   "start-cluster-agent",
	Short:  "Starts the cluster agent.",
	Long:   "Starts the cluster agent.",
}

func runCmdStartClusterAgent(ctx *context.T, env *cmdline.Env, args []string, config *vkubeConfig) error {
	if err := createClusterAgent(ctx, config); err != nil {
		return err
	}
	if !flagWait {
		fmt.Fprintln(env.Stdout, "Starting cluster agent.")
		return nil
	}
	if err := watchDeploymentRollout(clusterAgentApplicationName, config.ClusterAgent.Namespace, flagWaitTimeout, env.Stdout); err != nil {
		return err
	}
	for {
		if _, err := findClusterAgent(config, true); err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	fmt.Fprintf(env.Stdout, "Cluster agent is ready.\n")
	return nil
}

var cmdStopClusterAgent = &cmdline.Command{
	Runner: kubeCmdRunner(runCmdStopClusterAgent),
	Name:   "stop-cluster-agent",
	Short:  "Stops the cluster agent.",
	Long:   "Stops the cluster agent.",
}

func runCmdStopClusterAgent(ctx *context.T, env *cmdline.Env, args []string, config *vkubeConfig) error {
	if err := stopClusterAgent(config); err != nil {
		return err
	}
	fmt.Fprintln(env.Stdout, "Stopping cluster agent.")
	return nil
}

var cmdUpdateClusterAgent = &cmdline.Command{
	Runner: kubeCmdRunner(runCmdUpdateClusterAgent),
	Name:   "update-cluster-agent",
	Short:  "Updates the cluster agent.",
	Long:   "Updates the cluster agent.",
}

func runCmdUpdateClusterAgent(ctx *context.T, env *cmdline.Env, args []string, config *vkubeConfig) error {
	if err := updateClusterAgent(config, env.Stdout, env.Stderr); err != nil {
		return err
	}
	if !flagWait {
		fmt.Fprintln(env.Stdout, "Updating cluster agent.")
	}
	if err := watchDeploymentRollout(clusterAgentApplicationName, config.ClusterAgent.Namespace, flagWaitTimeout, env.Stdout); err != nil {
		return err
	}
	for {
		if _, err := findClusterAgent(config, true); err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	fmt.Fprintf(env.Stdout, "Updated cluster agent is ready.\n")
	return nil
}

var cmdClaimClusterAgent = &cmdline.Command{
	Runner: kubeCmdRunner(runCmdClaimClusterAgent),
	Name:   "claim-cluster-agent",
	Short:  "Claims the cluster agent.",
	Long:   "Claims the cluster agent.",
}

func runCmdClaimClusterAgent(ctx *context.T, env *cmdline.Env, args []string, config *vkubeConfig) error {
	myBlessings, _ := v23.GetPrincipal(ctx).BlessingStore().Default()
	claimer := clusterAgentClaimer(config)
	if !myBlessings.CouldHaveNames([]string{claimer}) {
		return fmt.Errorf("principal isn't the expected claimer: got %q, expected %q", myBlessings, claimer)
	}
	extension := strings.TrimPrefix(config.ClusterAgent.Blessing, claimer+security.ChainSeparator)
	if err := claimClusterAgent(ctx, config, extension); err != nil {
		if verror.ErrorID(err) == verror.ErrUnknownMethod.ID {
			return fmt.Errorf("already claimed")
		}
		return err
	}
	fmt.Fprintln(env.Stdout, "Claimed cluster agent successfully.")
	return nil
}

var cmdUpdateConfig = &cmdline.Command{
	Runner: kubeCmdRunner(runCmdUpdateConfig),
	Name:   "update-config",
	Short:  "Updates vkube.cfg.",
	Long:   "Updates the vkube.cfg file with new cluster-agent and/or pod-agent images.",
}

func runCmdUpdateConfig(ctx *context.T, env *cmdline.Env, args []string, config *vkubeConfig) error {
	if flagClusterAgentImage != "" {
		if flagClusterAgentImage[0] == ':' {
			config.ClusterAgent.Image = removeTag(config.ClusterAgent.Image) + flagClusterAgentImage
		} else {
			config.ClusterAgent.Image = flagClusterAgentImage
		}
	}
	if flagPodAgentImage != "" {
		if flagPodAgentImage[0] == ':' {
			config.PodAgent.Image = removeTag(config.PodAgent.Image) + flagPodAgentImage
		} else {
			config.PodAgent.Image = flagPodAgentImage
		}
	}
	if err := writeConfig(flagConfigFile, config); err != nil {
		return err
	}
	fmt.Fprintf(env.Stdout, "Updated %q.\n", flagConfigFile)
	return nil
}

var cmdBuildDockerImages = &cmdline.Command{
	Runner: kubeCmdRunner(runCmdBuildDockerImages),
	Name:   "build-docker-images",
	Short:  "Builds the docker images for the cluster and pod agents.",
	Long:   "Builds the docker images for the cluster and pod agents.",
}

func runCmdBuildDockerImages(ctx *context.T, env *cmdline.Env, args []string, config *vkubeConfig) error {
	return buildDockerImages(config, flagTag, flagVerbose, env.Stdout)
}

var cmdCreateSecrets = &cmdline.Command{
	Runner: kubeCmdRunner(runCreateSecrets),
	Name:   "create-secrets",
	Short:  "Creates secrets",
	Long: `
Creates kubernetes secrets from the template files passed as arguments.

The --secrets flag points to an encrypted TAR file that contains the actual
secrets.

Each template file represents a kubernetes Secret object to be created by
substituting the template fields with the content of the files from the
encrypted TAR file.

Templates look like:

{
  "apiVersion": "v1",
  "kind": "Secret",
  "metadata": {
    "name": "secret-name"
  },
  "type": "Opaque",
  "data": {
    "file1": "{{base64 "path/file1"}}",
    "file2": "{{base64 "path/file2"}}"
  }
}

where path/file1 and path/file2 are files from the TAR file.
`,
	ArgsName: "<template> ...",
	ArgsLong: "<template> A file containing the template for the secret object.",
}

func runCreateSecrets(ctx *context.T, env *cmdline.Env, args []string, config *vkubeConfig) error {
	if len(args) == 0 {
		return env.UsageErrorf("must specify at least one template file")
	}
	if flagSecrets == "" {
		return env.UsageErrorf("must specify --secrets")
	}
	return createSecrets(flagSecrets, args)
}

var cmdUpgradeCluster = &cmdline.Command{
	Runner: kubeCmdRunner(runCmdUpgradeCluster),
	Name:   "upgrade-cluster",
	Short:  "Upgrade the cluster defined in vkube.cfg.",
	Long: `
Upgrade the cluster defined in vkube.cfg to the default latest version.

This command is only meaningful with Kubernetes clusters on the Google Container
Engine.
`,
}

func runCmdUpgradeCluster(ctx *context.T, env *cmdline.Env, args []string, config *vkubeConfig) error {
	if config.Project == "" {
		return env.UsageErrorf("upgrade-cluster requires 'project' to be set in vkube.cfg")
	}
	return upgradeCluster(ctx, config, env.Stdin, env.Stdout, env.Stderr)
}

var cmdKubectl = &cmdline.Command{
	Runner:   kubeCmdRunner(runCmdKubectl),
	Name:     "kubectl",
	Short:    "Runs kubectl on the cluster defined in vkube.cfg.",
	Long:     "Runs kubectl on the cluster defined in vkube.cfg.",
	ArgsName: "-- <kubectl args>",
	ArgsLong: "<kubectl args> are passed directly to the kubectl command.",
}

func runCmdKubectl(ctx *context.T, env *cmdline.Env, args []string, config *vkubeConfig) error {
	cmd := exec.Command(flagKubectlBin, args...)
	cmd.Stdin = env.Stdin
	cmd.Stdout = env.Stdout
	cmd.Stderr = env.Stderr
	return cmd.Run()
}

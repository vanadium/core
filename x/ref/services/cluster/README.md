# Vanadium Cluster

This package and its sub-packages implement the functionality required to
run Vanadium applications in a cluster environment.

In this kind of environment, the _application_ is composed of one or more
_replicas_. Each _replica_ has its own Principal with Blessings that are
extended from the _application_'s Blessings.

If the _application_'s Blessing is `root/my-service`, the _replicas_' Blessings
could be `root/my-service/XXX` and `root/my-service/YYY`.

The design was first proposed in [this document](https://docs.google.com/document/d/1fHZC9F3lyFo7v4bl5o9Bs1uNjE8-TvBD5PCl36UFlyg).

## Cluster agent

The Cluster agent is in charge of storing the Blessings of the _applications_.
When a replica starts, it fetches its Blessings from the cluster agent.

The Cluster agent interface is defined in [service.vdl](service.vdl).

## vkube

[vkube](vkube/doc.go) is a command-line tool that helps manage Vanadium applications
on [Kubernetes](http://kubernetes.io/).

Before *vkube* starts an application, it creates a new secret in the cluster
agent and saves the Secret Key in a Kubernetes Secret object. This Secret Key
will later be used by the _replicas_ to get their Blessings.

![New Secret Diagram](diagrams/new-secret.png?raw=true)

Then, it takes the user-specified [Deployment](http://kubernetes.io/docs/user-guide/deployments/)
and adds a [pod-agent](../agent/pod_agentd/doc.go) container to its Pod template.

The pod-agent uses the Secret Key to get the blessings for the _replica_, and
makes them available to the other container(s) via the agent protocol.

![Pod Agent Diagram](diagrams/pod-agent.png?raw=true)


# Walkthrough for GKE

This walkthrough shows everything you need to start a _test_ Vanadium
application on Kubernetes.

It assumes that you are using the Container Engine on Google Cloud (aka GKE).
The tools should work with Kubernetes on other platforms, but have not been
tested outside of GKE.

## Environment

This walkthrough assumes that the `JIRI_ROOT` is set and that
`$JIRI_ROOT/release/go/bin` is in your PATH.

```
# Our work directory
mkdir $HOME/cluster-walkthrough
cd $HOME/cluster-walkthrough

# For jiri and vanadium binaries
export PATH=$JIRI_ROOT/.jiri_root/scripts:$JIRI_ROOT/release/go/bin:$PATH

# Adjust these to your environment
export PROJECT=my-project
export ZONE=my-zone
export CLUSTER=my-cluster
```

## Tools

The following tools are required to work with GKE:

  * **gcloud**: follow the instructions at [cloud.google.com/sdk](https://cloud.google.com/sdk/)
  * **docker**: follow the instructions at [docker.com](https://www.docker.com/)
  * **kubectl**

```
gcloud components install kubectl
```

  * **vkube**

```
jiri go install v.io/x/ref/services/cluster/vkube
```

## Create a cluster

You can create a kubernetes cluster from the [cloud console](https://console.cloud.google.com/),
or with the _gcloud container clusters create_ command.

This command creates a cluster named `$CLUSTER`.

```
gcloud container clusters create $CLUSTER --num-nodes 1 --project $PROJECT --zone $ZONE
```

## Create a persistent disk for the cluster agent

```
gcloud compute disks create test-cluster-agent-disk --size 200GB --project $PROJECT --zone $ZONE
```

## Create test Vanadium credentials

```
jiri go install v.io/x/ref/cmd/principal
mkdir test-root
principal create --with-passphrase=false test-root test-root
export V23_CREDENTIALS=$(pwd)/test-root
principal dump
```

## Create vkube.cfg

By default, the **vkube** command looks for _vkube.cfg_ in the current
directory. If _vkube.cfg_ is not in your current directory, you can specify
where it is with _--config_.

```
 cat - <<EOF >vkube.cfg
{
  "project": "$PROJECT",
  "zone": "$ZONE",
  "cluster": "$CLUSTER",
  "clusterAgent": {
    "namespace": "default",
    "image": "gcr.io/$PROJECT/cluster-agent:latest",
    "cpu": "0.1",
    "memory": "250M",
    "blessing": "test-root:cluster-agent",
    "admin": "test-root",
    "persistentDisk": "test-cluster-agent-disk"
  },
  "podAgent": {
    "image": "gcr.io/$PROJECT/pod-agent:latest"
  }
}
EOF
```

The _blessings_ and _admin_ values should match the credentials that you
created earlier.

## Build the cluster-agent and pod-agent images

This command will build two docker images and push them to your project's
registry.

```
vkube build-docker-images -v
```

## Start the cluster agent

This command creates a _Service_ and a _Deployment_ for the cluster agent.
Make sure that `V23_CREDENTIALS` is set and points at your test credentials.

```
vkube start-cluster-agent --wait
```

## Claim the cluster agent

The first time that the cluster agent is started, it doesn't have any
blessings. The claim-cluster-agent command sends the blessings defined in
_vkube.cfg_ to the cluster agent.

```
vkube claim-cluster-agent
```

## Build a docker image

In this example, we use [fortuned](../../examples/fortune) as our test
application.

Let's create a docker image that contains a statically linked _fortuned_
binary, and push it to the project's registry.

```
mkdir fortune-docker

 cat - <<EOF >fortune-docker/Dockerfile
FROM busybox
COPY fortuned /usr/local/bin/
EOF

IMAGE=gcr.io/$PROJECT/fortuned
jiri go build -o fortune-docker/fortuned -ldflags "-extldflags -static" \
  v.io/x/ref/examples/fortune/fortuned
docker build -t $IMAGE fortune-docker
gcloud docker push $IMAGE
```

## Run your first application

### Create fortuned-deployment.json

The Pod template contains only the _fortuned_ container. There is no need to
specify the _pod-agent_ container. The *vkube* command adds it for us
transparently.

```
 cat - <<EOF >fortuned-deployment.json
{
  "apiVersion": "extensions/v1beta1",
  "kind": "Deployment",
  "metadata": {
    "name": "fortuned"
  },
  "spec": {
    "template": {
      "metadata": {
        "labels": {
          "application": "fortuned"
        }
      },
      "spec": {
        "containers": [
          {
            "name": "fortuned",
            "image": "gcr.io/$PROJECT/fortuned:latest",
            "command": [ "fortuned", "--v23.tcp.address=:2345" ],
            "ports": [ { "containerPort": 2345 } ]
          }
        ]
      }
    }
  }
}
EOF
```

### Start the application with blessing extension `fortuned`

```
vkube start -f fortuned-deployment.json fortuned --wait
```

### Verify that fortuned is running.

```
vkube kubectl get pods
```

### Create the fortune service (load balancer)

```
 cat - <<EOF >fortuned-service.json
{
  "apiVersion": "v1",
  "kind": "Service",
  "metadata": {
    "name": "fortune"
  },
  "spec": {
    "ports": [
      { "port": 2345, "targetPort": 2345 }
    ],
    "selector": {
      "application": "fortuned"
    },
    "type": "LoadBalancer"
  }
}
EOF

vkube kubectl create -f fortuned-service.json

vkube kubectl get service fortune
```

The last command will show the service's external IP address after a minute or
two.

### Verify that the service is working

```
jiri go install v.io/x/ref/cmd/vrpc

IPADDR=$(vkube kubectl get service fortune --template '{{range .status.loadBalancer.ingress}}{{.ip}}{{end}}')
vrpc signature /$IPADDR:2345
vrpc identify /$IPADDR:2345
```

This should show something like

```
$ vrpc signature /$IPADDR:2345
// Fortune is the interface to a fortune-telling service.
type "v.io/x/ref/examples/fortune".Fortune interface {
	// Adds a fortune to the set used by Get().
	Add(fortune string) error
	// Returns a random fortune.
	Get() (fortune string | error)
	// Returns whether or not a fortune exists.
	Has(fortune string) (bool | error)
}

$ vrpc identify /$IPADDR:2345
PRESENTED: test-root:fortuned:10.64.0.4:54945
VALID:     [test-root:fortuned:10.64.0.4:54945]
PUBLICKEY: 3d:92:fc:5f:b3:ee:72:1f:7b:bf:22:a2:d4:d8:86:1a
```

## Clean up

When you're done, these commands will turn off everything.

```
vkube stop -f fortuned-deployment.json
vkube kubectl delete service fortune
vkube stop-cluster-agent
gcloud compute disks delete test-cluster-agent-disk --project $PROJECT --zone $ZONE
```

If you created a test cluster, you can delete it with `gcloud container clusters create ...`

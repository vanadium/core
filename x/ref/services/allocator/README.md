# Allocator service

Allocator allows users to create an instance of a pre-defined service, e.g.
syncbase, that they can use for testing purposes.

It leverages the functionality provided by the [cluster package](../cluster) to
run the service on [Kubernetes](http://kubernetes.io/).

The allocator [server API](service.vdl) has 3 methods:

* Create()
* Destroy()
* List()

## Create()

Create is used to create a new instance of the service.

When the server receives the `Create()` request, it creates a new Kubernetes
Deployment that is based on a pre-defined template. The template is expanded
with the following data:

* `Name`: The name of the kubernetes Deployment and its persistent disk.
* `AccessList`: A JSON-encoded AccessList that contains the caller's blessing names.
* `MountName`: The name where the new instance should publish itself.
* `CreatorInfo`: A JSON-encoded string describing the caller, suitable to be used as an annotation.
* `OwnerHash`: A hash of the creator's email address.

## Destroy()

Destroy destroys the instance whose name is passed as argument, along with its
persistent disk.

## List()

List shows all the instances that are owned by the calling user.

# Example

## Tools

Refer to the [Tools](../cluster/README.md#tools) and
[Create a cluster](../cluster/README.md#create-a-cluster) sections of the
[Vanadium Cluster](../cluster) documentation.

This document assumes that the Kubernetes cluster already exists and that we
have a vkube.cfg file for it.

## Deployment template

Allocatord doesn't know anything about the service to allocate. Instead, it
takes a user-defined template as argument.

Let's use syncbase as a simplified example.

```
 cat - <<EOF> syncbased-deployment.json-template
{
  "apiVersion": "extensions/v1beta1",
  "kind": "Deployment",
  "metadata": {
    "name": "{{.Name}}",
    "annotations": {
      "v.io/creator-info": {{.CreatorInfo}}
    },
    "labels": {
      "ownerHash": "{{.OwnerHash}}"
    }
  },
  "spec": {
    "template": {
      "metadata": {
        "labels": {
          "application": "{{.Name}}"
        }
      },
      "spec": {
        "containers": [
          {
            "name": "syncbased",
            "image": "b.gcr.io/vanadium-docker-images/syncbased:latest"
            "command": [
              "syncbased",
              "--name={{.MountName}}",
              "--root-dir=/data/root",
              "--v23.permissions.literal={\"Admin\":{{.AccessList}}}"
            ],
            "volumeMounts": [ { "name": "data-disk", "mountPath": "/data" } ]
          }
        ],
        "volumes": [
          { "name": "data-disk", "gcePersistentDisk": { "pdName": "{{.Name}}", "fsType": "ext4" } }
        ]
      }
    }
  }
}
EOF
```

## Run allocatord

```
allocatord \
  --deployment-template=syncbased-deployment.json-template \
  --name=syncbase-allocator \
  --server-name=syncbased \
  --vkube-cfg=vkube.cfg
```

## Creating a new service instance

```
allocator --allocator=syncbase-allocator create
```

This command returns the name of the newly created syncbase instance.


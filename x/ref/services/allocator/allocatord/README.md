# Run allocatord locally

## Prerequisites

1.  [Jiri][1] set up and `JIRI_ROOT` environment variable set.
2.  [Google Cloud SDK][2] installed, so that `gcloud` and `gsutil`
    commands are on the `PATH`
3.  Kubernates control installed with

    ```sh
    gcloud components install kubectl
    ```
4.  You are added as an editor on the [vanadium-staging][3] project.

## Get oauth credentials file.

```sh
gsutil cp gs://vanadium-backup/allocatord_oauth.json /tmp/oauth.json
```

## Prepare.

```sh
source ${JIRI_ROOT}/infrastructure/scripts/util/vanadium-oncall.sh
jiri go install \
  v.io/x/ref/services/allocator/allocatord \
  v.io/x/ref/services/agent/vbecome \
  v.io/x/ref/services/cluster/vkube \
  v.io/x/ref/services/agent \
  v.io/x/ref/services/agent/v23agentd
agent -s on
```

## Run allocatord

```sh
as_service ${JIRI_ROOT}/release/go/bin/vbecome --name=allocatord \
${JIRI_ROOT}/release/go/bin/allocatord \
 --http-addr=:8166 \
 --external-url=http://localhost:8166 \
 --oauth-client-creds-file=/tmp/oauth.json \
 --secure-cookies=false \
 --deployment-template=${JIRI_ROOT}/infrastructure/gke/syncbase-users-staging/conf/syncbased-deployment.json-template \
 --server-name=syncbased \
 --max-instances-per-user=3 \
 --vkube-cfg=${JIRI_ROOT}/infrastructure/gke/syncbase-users-staging/vkube.cfg \
 --dashboard-gcm-metric=cloud-syncbase \
 --dashboard-gcm-project=vanadium-staging \
 --assets=${JIRI_ROOT}/release/go/src/v.io/x/ref/services/allocator/allocatord/assets \
 --static-assets-prefix=https://static.staging.v.io
```

and then visit http://localhost:8166 to browse the UI.

## Edit UI

To update UI, edit template/CSS/JavaScript files in the "assets"
directory and refresh the page to see the changes. No need to restart
allocatord.

Re-generate "assets/assets.go" file when UI changes are ready to be reviewed.

```sh
jiri go generate v.io/x/ref/services/allocator/allocatord
```


[1]: https://github.com/vanadium/go.jiri
[2]: https://cloud.google.com/sdk/downloads
[3]: https://pantheon.corp.google.com/iam-admin/iam/project?project=vanadium-staging

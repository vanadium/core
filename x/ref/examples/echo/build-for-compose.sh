#!/bin/bash

BIN=./bin
mkdir -p $BIN

set -e

build_container() {
	image=$1
	package=$2
	dockerfile=$3
	GOOS=linux GOARCH=amd64 go build -o ${image} ${package}
	create_dockerfile Dockerfile.$image $image
	docker build -q -f Dockerfile.$image --tag=${image} .
}

create_dockerfile() {
	output=$1
	binary=$2
	cat > $1 <<!
FROM alpine
COPY ${binary} /bin/${binary}
COPY creds/ /bin/creds/
ENTRYPOINT ["/bin/${binary}"]
!
}

DOCKERFILE=Dockerfile
cd ${BIN}

dpull() {
	image="$1"
	found=$(docker images --format="{{.Repository}}:{{.Tag}}" | grep "$image")
	if [[ -z "$found" ]]; then
		docker pull $image
	fi
}

dpull amazon/aws-xray-daemon:latest
dpull amazon/amazon-ec2-metadata-mock:v1.8.1

if [[ ! -d creds ]]; then
	go run v.io/x/ref/cmd/principal create --with-passphrase=false ./creds echo_cluster
fi

build_container mounttabled v.io/x/ref/services/mounttable/mounttabled ${DOCKERFILE}
build_container echo v.io/x/ref/examples/echo/echo ${DOCKERFILE}
build_container echod v.io/x/ref/examples/echo/echod ${DOCKERFILE}

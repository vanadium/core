#!/bin/bash

# This script creates the appropriate vanadium credentials for the simple client/server echo
# example.
go run v.io/x/ref/cmd/principal create --with-passphrase=false --overwrite ./creds-server echo-server
go run v.io/x/ref/cmd/principal create --with-passphrase=false --overwrite ./creds-client echo-client
go run v.io/x/ref/cmd/principal bless --v23.credentials=./creds-server  --for=24h ./creds-client friend:echo-client  |
	go run v.io/x/ref/cmd/principal --v23.credentials=./creds-client set forpeer - echo-server

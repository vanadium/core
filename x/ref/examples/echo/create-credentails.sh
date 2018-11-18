#!/bin/bash

# This script creates the appropriate vanadium credentials for the simple client/server echo
# example.
principal create --with-passphrase=false --overwrite ./creds-server echo-server
principal create --with-passphrase=false --overwrite ./creds-client echo-client
principal bless --v23.credentials=../creds-server  --for=24h ./creds-client friend:echo-client |
	principal --v23.credentials=./creds-client set forpeer - echo-server

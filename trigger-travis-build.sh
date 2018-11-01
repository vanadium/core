#!/bin/bash

token="$1"
owner="$2"
repo="$3"

if [[ -z "$token" ]]; then
    echo "no token specified"
    exit 1
if
if [[ -z "$owner" ]]; then
    echo "no owner specified"
    exit 1
if
if [[ -z "$repo" ]]; then
    echo "no repo specified"
    exit 1
if

body='{
"request": {
"branch":"master"
}}'

curl -s -X POST \
   -H "Content-Type: application/json" \
   -H "Accept: application/json" \
   -H "Travis-API-Version: 3" \
   -H "Authorization: token $token" \
   -d "$body" \
   "https://api.travis-ci.org/repo/${owner}%2F${repo}/requests"

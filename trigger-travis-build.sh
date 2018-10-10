body='{
"request": {
"branch":"master"
}}'

token="$1"
owner="$2"
repo="$3"
curl -v -s -X POST \
   -H "Content-Type: application/json" \
   -H "Accept: application/json" \
   -H "Travis-API-Version: 3" \
   -H "Authorization: token $token" \
   -d "$body" \
   "https://api.travis-ci.org/repo/${owner}%2F${repo}/requests"

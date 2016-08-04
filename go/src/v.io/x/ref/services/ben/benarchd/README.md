# benarchd

Benchmark result archival daemon. Usage in
[godoc](https://godoc.org/v.io/x/ref/services/ben/benarchd).

## Testing UI changes

```
# Build benarchd and some binaries to upload test data
jiri go install v.io/x/ref/services/ben/benarchd v.io/x/ref/services/ben/benup v.io/x/ref/cmd/principal v.io/x/ref/services/agent/v23agentd
cd ${JIRI_ROOT}/release/go/bin

# Create dummy credentials to run all processes with
export V23_CREDENTIALS=/tmp/bencreds
./principal create --with-passphrase=false "${V23_CREDENTIALS}" bentesting
./v23agentd

# Start benarchd
# And then visit http://localhost:14142 to browse the UI
# Edit HTML/CSS/JavaScript in internal/assets to mess around with the UI
./benarchd \
  --assets=${JIRI_ROOT}/release/go/src/v.io/x/ref/services/ben/benarchd/internal/assets \
  --v23.tcp.address=localhost:14141 \
  --http=localhost:14142 &

# Populate with data (repeat multiple times or use multiple packages for more data)
go test -run NONE -bench . crypto/sha256 | ./benup --archiver=/127.0.0.1:14141

# Cleanup when done
kill $!
./v23agentd stop
unset V23_CREDENTIALS
```

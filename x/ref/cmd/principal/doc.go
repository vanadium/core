// Copyright 2018 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
Command principal creates and manages Vanadium principals and blessings.

All objects are printed using base64url-vom-encoding.

Usage:
   principal [flags] <command>

The principal commands are:
   create        Create a new principal and persist it into a directory
   fork          Fork a new principal from the principal that this tool is
                 running as and persist it into a directory
   seekblessings Seek blessings from a web-based Vanadium blessing service
   recvblessings Receive blessings sent by another principal and use them as the
                 default
   dump          Dump out information about the principal
   dumpblessings Dump out information about the provided blessings
   dumproots     Dump out blessings of the identity providers of blessings
   blessself     Generate a self-signed blessing
   bless         Bless another principal
   set           Mutate the principal's blessings.
   get           Read the principal's blessings.
   recognize     Add to the set of identity providers recognized by this
                 principal
   union         Merge multiple blessings into one
   help          Display help for commands or topics

The global flags are:
 -alsologtostderr=true
   log to standard error as well as files
 -log_backtrace_at=:0
   when logging hits line file:N, emit a stack trace
 -log_dir=
   if non-empty, write log files to this directory
 -logtostderr=false
   log to standard error instead of files
 -max_stack_buf_size=4292608
   max size in bytes of the buffer to use for logging stack traces
 -metadata=<just specify -metadata to activate>
   Displays metadata for the program and exits.
 -stderrthreshold=2
   logs at or above this threshold go to stderr
 -time=false
   Dump timing information to stderr before exiting the program.
 -v=0
   log level for V logs
 -v23.credentials=
   directory to use for storing security credentials
 -v23.i18n-catalogue=
   18n catalogue files to load, comma separated
 -v23.namespace.root=[/(dev.v.io:r:vprod:service:mounttabled)@ns.dev.v.io:8101]
   local namespace root; can be repeated to provided multiple roots
 -v23.permissions.file=
   specify a perms file as <name>:<permsfile>
 -v23.permissions.literal=
   explicitly specify the runtime perms as a JSON-encoded access.Permissions.
   Overrides all --v23.permissions.file flags
 -v23.proxy=
   object name of proxy service to use to export services across network
   boundaries
 -v23.proxy.limit=0
   max number of proxies to connect to when the policy is to connect to all
   proxies; 0 implies all proxies
 -v23.proxy.policy=
   policy for choosing from a set of available proxy instances
 -v23.tcp.address=
   address to listen on
 -v23.tcp.protocol=
   protocol to listen with
 -v23.virtualized.advertise-private-addresses=
   if set the process will also advertise its private addresses
 -v23.virtualized.disallow-native-fallback=false
   if set, a failure to detect the requested virtualization provider will result
   in an error, otherwise, native mode is used
 -v23.virtualized.dns.public-name=
   if set the process will use the supplied dns name (and port) without
   resolution for its entry in the mounttable
 -v23.virtualized.docker=
   set if the process is running in a docker container and needs to configure
   itself differently therein
 -v23.virtualized.provider=
   the name of the virtualization/cloud provider hosting this process if the
   process needs to configure itself differently therein
 -v23.virtualized.tcp.public-address=
   if set the process will use this address (resolving via dns if appropriate)
   for its entry in the mounttable
 -v23.virtualized.tcp.public-protocol=
   if set the process will use this protocol for its entry in the mounttable
 -v23.vtrace.cache-size=1024
   The number of vtrace traces to store in memory
 -v23.vtrace.collect-regexp=
   Spans and annotations that match this regular expression will trigger trace
   collection
 -v23.vtrace.dump-on-shutdown=true
   If true, dump all stored traces on runtime shutdown
 -v23.vtrace.sample-rate=0
   Rate (from 0.0 to 1.0) to sample vtrace traces
 -v23.vtrace.v=0
   The verbosity level of the log messages to be captured in traces
 -vmodule=
   comma-separated list of globpattern=N settings for filename-filtered logging
   (without the .go suffix).  E.g. foo/bar/baz.go is matched by patterns baz or
   *az or b* but not by bar/baz or baz.go or az or b.*
 -vpath=
   comma-separated list of regexppattern=N settings for file pathname-filtered
   logging (without the .go suffix).  E.g. foo/bar/baz.go is matched by patterns
   foo/bar/baz or fo.*az or oo/ba or b.z but not by foo/bar/baz.go or fo*az

Principal create - Create a new principal and persist it into a directory

Creates a new principal with a single optional self-blessed blessing and writes
it out to the provided directory. The same directory can then be used to set the
V23_CREDENTIALS environment variable for other vanadium applications.

The operation fails if the directory already contains a principal. In this case
the --overwrite flag can be provided to clear the directory and write out the
new principal.

Usage:
   principal create [flags] <directory> [<blessing>]

<directory> is the directory to which the new principal will be persisted.

<blessing> is the optional self-blessed blessing that the principal will be
setup to use by default.  If a blessing argument is not provided, the new
principal will have no blessings.

The principal create flags are:
 -key-type=ecdsa256
   The type of key to be created, allowed values are ecdsa256, ecdsa384,
   ecdsa521, ed25519.
 -overwrite=false
   If true, any existing principal data in the directory will be overwritten
 -ssh-public-key=
   If set, use the key hosted by the accessible ssh-agent that corresponds to
   the specified public key file.
 -with-passphrase=true
   If true, the user is prompted for a passphrase to encrypt the principal.
   Otherwise, the principal is stored unencrypted.

Principal fork - Fork a new principal from the principal that this tool is running as and persist it into a directory

Creates a new principal with a blessing from the principal specified by the
environment that this tool is running in, and writes it out to the provided
directory. The blessing that will be extended is the default one from the
blesser's store, or specified by the --with flag. Expiration on the blessing are
controlled via the --for flag. Additional caveats on the blessing are controlled
with the --caveat flag. The blessing is marked as default and shareable with all
peers on the new principal's blessing store.

The operation fails if the directory already contains a principal. In this case
the --overwrite flag can be provided to clear the directory and write out the
forked principal.

Usage:
   principal fork [flags] <directory> <extension>

<directory> is the directory to which the forked principal will be persisted.

<extension> is the extension under which the forked principal is blessed.

The principal fork flags are:
 -caveat=
   "package/path".CaveatName:VDLExpressionParam to attach to this blessing
 -for=0s
   Duration of blessing validity (zero implies no expiration)
 -key-type=ecdsa256
   The type of key to be created, allowed values are ecdsa256, ecdsa384,
   ecdsa521, ed25519.
 -overwrite=false
   If true, any existing principal data in the directory will be overwritten
 -require-caveats=true
   If false, allow blessing without any caveats. This is typically not advised
   as the principal wielding the blessing will be almost as powerful as its
   blesser
 -ssh-public-key=
   If set, use the key hosted by the accessible ssh-agent that corresponds to
   the specified public key file.
 -with=
   Path to file containing blessing to extend
 -with-passphrase=true
   If true, the user is prompted for a passphrase to encrypt the principal.
   Otherwise, the principal is stored unencrypted.

Principal seekblessings - Seek blessings from a web-based Vanadium blessing service

Seeks blessings from a web-based Vanadium blesser which requires the caller to
first authenticate with Google using OAuth. Simply run the command to see what
happens.

The blessings are sought for the principal specified by the environment that
this tool is running in.

The blessings obtained are set as default unless the --set-default flag is set
to false, and are also set for sharing with all peers unless a more specific
peer pattern is provided using the --for-peer flag.

Usage:
   principal seekblessings [flags]

The principal seekblessings flags are:
 -add-to-roots=true
   If true, the root certificate of the blessing will be added to the
   principal's set of recognized root certificates
 -browser=true
   If false, the seekblessings command will not open the browser and only print
   the url to visit.
 -for-peer=...
   If non-empty, the blessings obtained will be marked for peers matching this
   pattern in the store
 -from=https://dev.v.io/auth/google
   URL to use to begin the seek blessings process
 -set-default=true
   If true, the blessings received will be set as the default blessing in the
   store

Principal recvblessings - Receive blessings sent by another principal and use them as the default

Allow another principal (likely a remote process) to bless this one.

This command sets up the invoker (this process) to wait for a blessing from
another invocation of this tool (remote process) and prints out the command to
be run as the remote principal.

The received blessings are set as default unless the --set-default flag is set
to false, and are also set for sharing with all peers unless a more specific
peer pattern is provided using the --for-peer flag.

TODO(ashankar,cnicolaou): Make this next paragraph possible! Requires the
ability to obtain the proxied endpoint.

Typically, this command should require no arguments. However, if the sender and
receiver are on different network domains, it may make sense to use the
--v23.proxy flag:
    principal --v23.proxy=proxy recvblessings

The command to be run at the sender is of the form:
    principal bless --remote-key=KEY --remote-token=TOKEN ADDRESS EXTENSION

The --remote-key flag is used to by the sender to "authenticate" the receiver,
ensuring it blesses the intended recipient and not any attacker that may have
taken over the address.

The --remote-token flag is used by the sender to authenticate itself to the
receiver. This helps ensure that the receiver rejects blessings from senders who
just happened to guess the network address of the 'recvblessings' invocation.

If the --remote-arg-file flag is provided to recvblessings, the remote key,
remote token and object address of this principal will be written to the
specified location. This file can be supplied to bless:
    principal bless --remote-arg-file FILE EXTENSION

Usage:
   principal recvblessings [flags]

The principal recvblessings flags are:
 -for-peer=...
   If non-empty, the blessings obtained will be marked for peers matching this
   pattern in the store
 -remote-arg-file=
   If non-empty, the remote key, remote token, and principal will be written to
   the specified file in a JSON object. This can be provided to 'principal bless
   --remote-arg-file FILE EXTENSION'
 -set-default=true
   If true, the blessings received will be set as the default blessing in the
   store

Principal dump - Dump out information about the principal

Prints out information about the principal specified by the environment that
this tool is running in.

Usage:
   principal dump [flags]

The principal dump flags are:
 -s=false
   If true, show only the default blessing names

Principal dumpblessings - Dump out information about the provided blessings

Prints out information about the blessings (typically obtained from this tool)
encoded in the provided file.

Usage:
   principal dumpblessings [flags] <file>

<file> is the path to a file containing blessings typically obtained from this
tool. - is used for STDIN.

Principal dumproots - Dump out blessings of the identity providers of blessings

Prints out the blessings of the identity providers of the input blessings.  One
line per identity provider, each line is a base64url-encoded (RFC 4648, Section
5) vom-encoded Blessings object.

Usage:
   principal dumproots [flags] <file>

<file> is the path to a file containing blessings (base64url-encoded
vom-encoded). - is used for STDIN.

Principal blessself - Generate a self-signed blessing

Returns a blessing with name <name> and self-signed by the principal specified
by the environment that this tool is running in. Optionally, the blessing can be
restricted with an expiry caveat specified using the --for flag. Additional
caveats can be added with the --caveat flag.

Usage:
   principal blessself [flags] [<name>]

<name> is the name used to create the self-signed blessing. If not specified, a
name will be generated based on the hostname of the machine and the name of the
user running this command.

The principal blessself flags are:
 -caveat=
   "package/path".CaveatName:VDLExpressionParam to attach to this blessing
 -for=0s
   Duration of blessing validity (zero implies no expiration)

Principal bless - Bless another principal

Bless another principal.

The blesser is obtained from the runtime this tool is using. The blessing that
will be extended is the default one from the blesser's store, or specified by
the --with flag. Expiration on the blessing are controlled via the --for flag.
Additional caveats are controlled with the --caveat flag.

For example, let's say a principal "alice" wants to bless another principal
"bob" as "alice:friend", the invocation would be:
    V23_CREDENTIALS=<path to alice> principal bless <path to bob> friend
and this will dump the blessing to STDOUT.

With the --remote-key and --remote-token flags, this command can be used to
bless a principal on a remote machine. In this case, the blessing is not dumped
to STDOUT but sent to the remote end. Use 'principal help recvblessings' for
details.

When --remote-arg-file is specified, only the blessing extension is required, as
all other arguments will be extracted from the specified file.

Usage:
   principal bless [flags] [<principal to bless>] [<extension>]

<principal to bless> represents the principal to be blessed (i.e., whose public
key will be provided with a name).  This can be either:

(a) The directory containing credentials for that principal,
  OR
(b) The filename (- for STDIN) containing the base64url-encoded public
    key or any other blessings of the principal,
  OR
(c) The object name produced by the 'recvblessings' command of this tool
    running on behalf of another principal (if the --remote-key and
    --remote-token flags are specified).
  OR
(d) None (if the --remote-arg-file flag is specified, only <extension> should
    be provided to bless).

<extension> is the string extension that will be applied to create the blessing.

The principal bless flags are:
 -caveat=
   "package/path".CaveatName:VDLExpressionParam to attach to this blessing
 -for=0s
   Duration of blessing validity (zero implies no expiration)
 -remote-arg-file=
   File containing bless arguments written by 'principal recvblessings
   -remote-arg-file FILE EXTENSION' command. This can be provided to bless in
   place of --remote-key, --remote-token, and <principal>
 -remote-key=
   Public key of the remote principal to bless (obtained from the
   'recvblessings' command run by the remote principal
 -remote-token=
   Token provided by principal running the 'recvblessings' command
 -require-caveats=true
   If false, allow blessing without any caveats. This is typically not advised
   as the principal wielding the blessing will be almost as powerful as its
   blesser
 -with=
   Path to file containing blessing to extend

Principal set

Commands to mutate the blessings of the principal.

All input blessings are expected to be serialized using base64url-vom-encoding.
See 'principal get'.

Usage:
   principal set [flags] <command>

The principal set commands are:
   default     Set provided blessings as default
   forpeer     Set provided blessings for peer

Principal set default - Set provided blessings as default

Sets the provided blessings as default in the BlessingStore specified by the
environment that this tool is running in.

It is an error to call 'set default' with blessings whose public key does not
match the public key of the principal specified by the environment.

Usage:
   principal set default [flags] <file>

<file> is the path to a file containing a blessing typically obtained from this
tool. - is used for STDIN.

The principal set default flags are:
 -add-to-roots=true
   If true, the root certificate of the blessing will be added to the
   principal's set of recognized root certificates

Principal set forpeer - Set provided blessings for peer

Marks the provided blessings to be shared with the provided peers on the
BlessingStore specified by the environment that this tool is running in.

'set b pattern' marks the intention to reveal b to peers who present blessings
of their own matching 'pattern'.

'set nil pattern' can be used to remove the blessings previously associated with
the pattern (by a prior 'set' command).

It is an error to call 'set forpeer' with blessings whose public key does not
match the public key of this principal specified by the environment.

Usage:
   principal set forpeer [flags] <file> <pattern>

<file> is the path to a file containing a blessing typically obtained from this
tool. - is used for STDIN.

<pattern> is the BlessingPattern used to identify peers with whom this blessing
can be shared with.

The principal set forpeer flags are:
 -add-to-roots=true
   If true, the root certificate of the blessing will be added to the
   principal's set of recognized root certificates

Principal get

Commands to inspect the blessings of the principal.

All blessings are printed to stdout using base64url-vom-encoding.

Usage:
   principal get [flags] <command>

The principal get commands are:
   default         Return blessings marked as default
   forpeer         Return blessings marked for the provided peer
   publickey       Prints the public key of the principal.
   recognizedroots Return recognized blessings, and their associated public key.
   peermap         Shows the map from peer pattern to which blessing name to
                   present.

Principal get default - Return blessings marked as default

Returns blessings that are marked as default in the BlessingStore specified by
the environment that this tool is running in. Providing --names will print the
default blessings' chain names. Providing --rootkey <chain_name> will print the
root key of the certificate chain with chain_name. Providing --caveats
<chain_name> will print the caveats on the certificate chain with chain_name.

Usage:
   principal get default [flags]

The principal get default flags are:
 -caveats=
   Shows the caveats on the provided certificate chain name.
 -names=false
   If true, shows the value of the blessing name to be presented to the peer
 -rootkey=
   Shows the value of the root key of the provided certificate chain name.

Principal get forpeer - Return blessings marked for the provided peer

Returns blessings that are marked for the provided peer in the BlessingStore
specified by the environment that this tool is running in. Providing --names
will print the blessings' chain names. Providing --rootkey <chain_name> will
print the root key of the certificate chain with chain_name. Providing --caveats
<chain_name> will print the caveats on the certificate chain with chain_name.

Usage:
   principal get forpeer [flags] [<peer_1> ... <peer_k>]

<peer_1> ... <peer_k> are the (human-readable string) blessings bound to the
peer. The returned blessings are marked with a pattern that is matched by at
least one of these. If no arguments are specified, store.forpeer returns the
blessings that are marked for all peers (i.e., blessings set on the store with
the "..." pattern).

The principal get forpeer flags are:
 -caveats=
   Shows the caveats on the provided certificate chain name.
 -names=false
   If true, shows the value of the blessing name to be presented to the peer
 -rootkey=
   Shows the value of the root key of the provided certificate chain name.

Principal get publickey

Prints out the public key of the principal specified by the environment that
this tool is running in.

The key is printed as a base64url encoded bytes (RFC 4648, Section 5) of the
DER-format representation of the key (suitable to be provided as an argument to
the 'recognize' command for example).

With --pretty, a 16-byte fingerprint of the key instead. This format is easier
for humans to read and is used in output of other commands in this program, but
is not suitable as an argument to the 'recognize' command.

Usage:
   principal get publickey [flags]

The principal get publickey flags are:
 -pretty=false
   If true, print the key out in a more human-readable but lossy representation.

Principal get recognizedroots

Shows list of blessing names that the principal recognizes, and their associated
public key. If the principal is operating as a client, contacted servers must
appear on this list. If the principal is operating as a server, clients must
present blessings derived from this list.

Usage:
   principal get recognizedroots [flags]

Principal get peermap

Shows the map from peer pattern to which blessing name to present. If the
principal operates as a server, it presents its default blessing to all peers.
If the principal operates as a client, it presents the map value associated with
the peer it contacts.

Usage:
   principal get peermap [flags]

Principal recognize - Add to the set of identity providers recognized by this principal

Adds an identity provider to the set of recognized root public keys for this
principal.

It accepts either a single argument (which points to a file containing a
blessing) or two arguments (a name and a base64url-encoded DER-encoded public
key).

For example, to make the principal in credentials directory A recognize the root
of the default blessing in credentials directory B:
  principal -v23.credentials=B bless A some_extension |
  principal -v23.credentials=A recognize -
The extension 'some_extension' has no effect in the command above.

Or to make the principal in credentials directory A recognize the public key for
the principal in credentials directory B for blessing pattern P:
  principal -v23.credentials=A recognize P $(principal -v23.credentials=B get publickey)

Usage:
   principal recognize [flags] <blessing pattern|blessing> [<key>]

<blessing> is the path to a file containing a blessing typically obtained from
this tool. - is used for STDIN.

<blessing pattern> is the blessing pattern for which <key> should be recognized.

<key> is a base64url-encoded, DER-encoded public key, such as that printed by
"principal get publickey".

Principal union - Merge multiple blessings into one

Merges multiple blessings into one.

It accepts multiple base64url-encoded blessings. Each argument can be a file
containing a blessing, or the blessing itself. It returns the union of all the
blessings.

For example, to merge the blessings contained in files A and B:
  principal union A B, or
  principal union $(cat A) $(cat B)

Usage:
   principal union [flags] [<blessing> | <blessing file>...]

<blessing> is a base64url-encoded blessing.

<blessing file> is a file that contains a base64url-encoded blessing.

Principal help - Display help for commands or topics

Help with no args displays the usage of the parent command.

Help with args displays the usage of the specified sub-command or help topic.

"help ..." recursively displays help for all commands and topics.

Usage:
   principal help [flags] [command/topic ...]

[command/topic ...] optionally identifies a specific sub-command or help topic.

The principal help flags are:
 -style=compact
   The formatting style for help output:
      compact   - Good for compact cmdline output.
      full      - Good for cmdline output, shows all global flags.
      godoc     - Good for godoc processing.
      shortonly - Only output short description.
   Override the default by setting the CMDLINE_STYLE environment variable.
 -width=<terminal width>
   Format output to this target width in runes, or unlimited if width < 0.
   Defaults to the terminal width if available.  Override the default by setting
   the CMDLINE_WIDTH environment variable.
*/
package main

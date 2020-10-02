// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run v.io/x/lib/cmdline/gendoc .

package main

import (
	"bytes"
	gocontext "context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"strings"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/vom"
	"v.io/x/lib/cmdline"
	"v.io/x/ref"
	vsecurity "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/security/passphrase"
	"v.io/x/ref/lib/security/signing/sshagent"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/static"
)

// Flags common to many commands

// CaveatFlag represents a --caveat flag.
type CaveatFlag struct {
	Caveat caveatsFlag `cmdline:"caveat,,\"package/path\".CaveatName:VDLExpressionParam to attach to this blessing"`
}

// CaveatsFlag represents a --caveats flag.
type CaveatsFlag struct {
	Caveats string `cmdline:"caveats,,'Shows the caveats on the provided certificate chain name.'"`
}

// ForFlag represents a --for flag.
type ForFlag struct {
	For time.Duration `cmdline:"for,0,Duration of blessing validity (zero implies no expiration)"`
}

// WithFlag represents a --with flag.
type WithFlag struct {
	With string `cmdline:"with,,Path to file containing blessing to extend"`
}

// AddToRootsFlag represents a --add-to-roots flag.
type AddToRootsFlag struct {
	AddToRoots bool `cmdline:"add-to-roots,true,'If true, the root certificate of the blessing will be added to the principal\\'s set of recognized root certificates'"`
}

// CreateOverwriteFlag represents a --overwrite flag.
type CreateOverwriteFlag struct {
	CreateOverwrite bool `cmdline:"overwrite,,'If true, any existing principal data in the directory will be overwritten'"`
}

// WithPassphraseFlag represent a --with-passphrase flag.
type WithPassphraseFlag struct {
	WithPassphrase bool `cmdline:"with-passphrase,true,'If true, the user is prompted for a passphrase to encrypt the principal. Otherwise, the principal is stored unencrypted.'"`
}

// KeyFlags represents the flag used to specify the type of key to generate/use
// for a new principal.
type KeyFlags struct {
	KeyType               string `cmdline:"key-type,ecdsa256,'The type of key to be created, allowed values are ecdsa256, ecdsa384, ecdsa521, ed25519.'"`
	SSHAgentPublicKeyFile string `cmdline:"ssh-public-key,,'If set, use the key hosted by the accessible ssh-agent that corresponds to the specified public key file.'"`
}

// ForPeerFlag represents a --for-peer flag.
type ForPeerFlag struct {
	ForPeer string `cmdline:"for-peer,,'If non-empty, the blessings obtained will be marked for peers matching this pattern in the store'"`
}

// BlessingsRootKeyFlag represents a --rootkey flag.
type BlessingsRootKeyFlag struct {
	RootKey string `cmdline:"rootkey,,'Shows the value of the root key of the provided certificate chain name.'"`
}

// NamesFlag represents a --name flag.
type NamesFlag struct {
	Names bool `cmdline:"names,false,'If true, shows the value of the blessing name to be presented to the peer'"`
}

// SetDefaultFlag represents a --set-default flag.
type SetDefaultFlag struct {
	SetDefault bool `cmdline:"set-default,true,'If true, the blessings received will be set as the default blessing in the store'"`
}

func defaultBlessingFrom() string {
	if e := os.Getenv(ref.EnvOAuthIdentityProvider); e != "" {
		return e
	}
	return "https://dev.v.io/auth/google"
}

var (

	// Flags for the "blessself" command
	flagBlessSelf = struct {
		CaveatFlag
		ForFlag
	}{}
	flagBlessSelfDef = cmdline.FlagDefinitions{Flags: &flagBlessSelf}

	// Flags for the "bless" command
	flagBless = struct {
		CaveatFlag
		ForFlag
		WithFlag
		RemoteArgFile  string `cmdline:"remote-arg-file,,'File containing bless arguments written by \\'principal recvblessings -remote-arg-file FILE EXTENSION\\' command. This can be provided to bless in place of --remote-key, --remote-token, and <principal>'"`
		RequireCaveats bool   `cmdline:"require-caveats,true,'If false, allow blessing without any caveats. This is typically not advised as the principal wielding the blessing will be almost as powerful as its blesser'"`
		RemoteKey      string `cmdline:"remote-key,,Public key of the remote principal to bless (obtained from the 'recvblessings' command run by the remote principal"`
		RemoteToken    string `cmdline:"remote-token,,Token provided by principal running the 'recvblessings' command"`
	}{}
	flagBlessDef = cmdline.FlagDefinitions{Flags: &flagBless}

	// Flags for the "fork" command
	flagFork = struct {
		CaveatFlag
		ForFlag
		WithFlag
		CreateOverwriteFlag
		WithPassphraseFlag
		KeyFlags
		RequireCaveats bool `cmdline:"require-caveats,true,'If false, allow blessing without any caveats. This is typically not advised as the principal wielding the blessing will be almost as powerful as its blesser'"`
	}{}
	flagForkDef = cmdline.FlagDefinitions{Flags: &flagFork}

	// Flags for the "seekblessings" command
	flagSeekBlessings = struct {
		AddToRootsFlag
		ForPeerFlag
		From string `cmdline:"from,,URL to use to begin the seek blessings process"`
		SetDefaultFlag
		Browser bool `cmdline:"browser,true,'If false, the seekblessings command will not open the browser and only print the url to visit.'"`
	}{}
	flagSeekBlessingsDef = cmdline.FlagDefinitions{
		Flags: &flagSeekBlessings,
		ValueDefaults: map[string]interface{}{
			"for-peer": string(security.AllPrincipals),
			"from":     defaultBlessingFrom(),
		},
	}

	// Flags for the "recvblessings" command
	flagRecvBlessings = struct {
		ForPeerFlag
		SetDefaultFlag
		RemoteArgFile string `cmdline:"remote-arg-file,,'If non-empty, the remote key, remote token, and principal will be written to the specified file in a JSON object. This can be provided to \\'principal bless --remote-arg-file FILE EXTENSION\\''"`
	}{}
	flagRecvBlessingsDef = cmdline.FlagDefinitions{
		Flags: &flagRecvBlessings,
		ValueDefaults: map[string]interface{}{
			"for-peer": string(security.AllPrincipals),
		},
	}

	// Flags for the "set forpeer" command
	flagSetForPeer = struct {
		AddToRootsFlag
	}{}
	flagSetForPeerDef = cmdline.FlagDefinitions{Flags: &flagSetForPeer}

	// Flags for the "get forpeer" command
	flagGetForPeer = struct {
		CaveatsFlag
		BlessingsRootKeyFlag
		NamesFlag
	}{}
	flagGetForPeerDef = cmdline.FlagDefinitions{Flags: &flagGetForPeer}

	// Flags for the 'get defaults' command
	flagGetDefaults = struct {
		NamesFlag
		BlessingsRootKeyFlag
		CaveatsFlag
	}{}
	flagGetDefaultsDef = cmdline.FlagDefinitions{Flags: &flagGetDefaults}

	// Flags for the 'set defaults' command
	flagSetDefaults = struct {
		AddToRootsFlag
	}{}
	flagSetDefaultsDef = cmdline.FlagDefinitions{Flags: &flagSetDefaults}

	//  Flags for the 'create' command
	flagCreate = struct {
		CreateOverwriteFlag
		WithPassphraseFlag
		KeyFlags
	}{}
	flagCreateDef = cmdline.FlagDefinitions{Flags: &flagCreate}

	// Flags for the get publickey command.
	flagGetPublicKey = struct {
		Pretty bool `cmdline:"pretty,,'If true, print the key out in a more human-readable but lossy representation.'"`
	}{}
	flagGetPublicKeyDef = cmdline.FlagDefinitions{Flags: &flagGetPublicKey}

	// Flags for the dump command.
	flagDumpFlags = struct {
		Short bool `cmdline:"s,false,'If true, show only the default blessing names'"`
	}{}
	flagDumpDef = cmdline.FlagDefinitions{Flags: &flagDumpFlags}

	errNoCaveats = fmt.Errorf("no caveats provided: it is generally dangerous to bless another principal without any caveats as that gives them almost unrestricted access to the blesser's credentials. If you really want to do this, set --require-caveats=false")

	cmdDump = &cmdline.Command{
		Name:  "dump",
		Short: "Dump out information about the principal",
		Long: `
Prints out information about the principal specified by the environment
that this tool is running in.
`,
		FlagDefs: flagDumpDef,
		Runner: v23cmd.RunnerFunc(func(ctx *context.T, env *cmdline.Env, args []string) error {
			p := v23.GetPrincipal(ctx)
			def, _ := p.BlessingStore().Default()
			if flagDumpFlags.Short {
				fmt.Printf("%s\n", printAnnotatedBlessingsNames(def))
				return nil
			}
			fmt.Printf("Public key : %v\n", p.PublicKey())
			// NOTE(caprita): We print the default blessings name
			// twice (it's also printed as part of the blessing
			// store below) -- the reason we print it here is to
			// expose whether the blessings are expired.  Ideally,
			// the blessings store would print the expiry
			// information about each blessing in the store, but
			// that would require deeper changes beyond the
			// principal tool.
			fmt.Printf("Default Blessings : %s\n", printAnnotatedBlessingsNames(def))
			fmt.Println("---------------- BlessingStore ----------------")
			fmt.Printf("%v", p.BlessingStore().DebugString())
			fmt.Println("---------------- BlessingRoots ----------------")
			fmt.Printf("%v", p.Roots().DebugString())
			return nil
		}),
	}

	cmdDumpBlessings = &cmdline.Command{
		Name:  "dumpblessings",
		Short: "Dump out information about the provided blessings",
		Long: `
Prints out information about the blessings (typically obtained from this tool)
encoded in the provided file.
`,
		ArgsName: "<file>",
		ArgsLong: `
<file> is the path to a file containing blessings typically obtained from
this tool. - is used for STDIN.
`,
		Runner: cmdline.RunnerFunc(func(env *cmdline.Env, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("requires exactly one argument, <file>, provided %d", len(args))
			}
			blessings, err := decodeBlessings(args[0])
			if err != nil {
				return fmt.Errorf("failed to decode provided blessings: %v", err)
			}
			wire, err := blessings2wire(blessings)
			if err != nil {
				return fmt.Errorf("failed to decode certificate chains: %v", err)
			}
			fmt.Printf("Blessings          : %s\n", printAnnotatedBlessingsNames(blessings))
			fmt.Printf("PublicKey          : %v\n", blessings.PublicKey())
			fmt.Printf("Certificate chains : %d\n", len(wire.CertificateChains))
			for idx, chain := range wire.CertificateChains {
				fmt.Printf("Chain #%d (%d certificates). Root certificate public key: %v\n", idx, len(chain), rootkey(chain))
				for certidx, cert := range chain {
					fmt.Printf("  Certificate #%d: %v with ", certidx, cert.Extension)
					switch n := len(cert.Caveats); n {
					case 1:
						fmt.Printf("1 caveat")
					default:
						fmt.Printf("%d caveats", n)
					}
					fmt.Println("")
					for cavidx, cav := range cert.Caveats {
						fmt.Printf("    (%d) %v\n", cavidx, cav.String())
					}
				}
			}
			return nil
		}),
	}

	cmdDumpRoots = &cmdline.Command{
		Name:  "dumproots",
		Short: "Dump out blessings of the identity providers of blessings",
		Long: `
Prints out the blessings of the identity providers of the input blessings.  One
line per identity provider, each line is a base64url-encoded (RFC 4648, Section
5) vom-encoded Blessings object.
`,
		ArgsName: "<file>",
		ArgsLong: `
<file> is the path to a file containing blessings (base64url-encoded vom-encoded).
- is used for STDIN.
`,
		Runner: cmdline.RunnerFunc(func(env *cmdline.Env, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("requires exactly one argument, <file>, provided %d", len(args))
			}
			blessings, err := decodeBlessings(args[0])
			if err != nil {
				return fmt.Errorf("failed to decode provided blesseings: %v", err)
			}
			for _, root := range security.RootBlessings(blessings) {
				if err := dumpBlessings(root); err != nil {
					return err
				}
			}
			return nil
		}),
	}

	cmdBlessSelf = &cmdline.Command{
		Name:  "blessself",
		Short: "Generate a self-signed blessing",
		Long: `
Returns a blessing with name <name> and self-signed by the principal specified
by the environment that this tool is running in. Optionally, the blessing can
be restricted with an expiry caveat specified using the --for flag. Additional
caveats can be added with the --caveat flag.
`,
		ArgsName: "[<name>]",
		ArgsLong: `
<name> is the name used to create the self-signed blessing. If not
specified, a name will be generated based on the hostname of the
machine and the name of the user running this command.
`,
		FlagDefs: flagBlessSelfDef,
		Runner: v23cmd.RunnerFunc(func(ctx *context.T, env *cmdline.Env, args []string) error {
			var name string
			switch len(args) {
			case 0:
				name = defaultBlessingName()
			case 1:
				name = args[0]
			default:
				return fmt.Errorf("requires at most one argument, provided %d", len(args))
			}
			caveats, err := caveatsFromFlags(flagBlessSelf.For, &flagBlessSelf.Caveat)
			if err != nil {
				return err
			}
			principal, err := getMutablePrincipal(root)
			if err != nil {
				return err
			}
			blessing, err := principal.BlessSelf(name, caveats...)
			if err != nil {
				return fmt.Errorf("failed to create self-signed blessing for name %q: %v", name, err)
			}

			return dumpBlessings(blessing)
		}),
	}

	cmdBless = &cmdline.Command{
		Name:  "bless",
		Short: "Bless another principal",
		Long: `
Bless another principal.

The blesser is obtained from the runtime this tool is using. The blessing that
will be extended is the default one from the blesser's store, or specified by
the --with flag. Expiration on the blessing are controlled via the --for flag.
Additional caveats are controlled with the --caveat flag.

For example, let's say a principal "alice" wants to bless another principal "bob"
as "alice:friend", the invocation would be:
    V23_CREDENTIALS=<path to alice> principal bless <path to bob> friend
and this will dump the blessing to STDOUT.

With the --remote-key and --remote-token flags, this command can be used to
bless a principal on a remote machine. In this case, the blessing is not dumped
to STDOUT but sent to the remote end. Use 'principal help recvblessings' for
details.

When --remote-arg-file is specified, only the blessing extension is required, as all other
arguments will be extracted from the specified file.
`,
		ArgsName: "[<principal to bless>] [<extension>]",
		ArgsLong: `
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

<extension> is the string extension that will be applied to create the
blessing.

	`,
		FlagDefs: flagBlessDef,
		Runner: v23cmd.RunnerFunc(func(ctx *context.T, env *cmdline.Env, args []string) error {
			if len(flagBless.RemoteArgFile) > 0 {
				if len(args) > 1 {
					return fmt.Errorf("when --remote-arg-file is provided, only <extension> is expected, provided %d", len(args))
				}
				if (len(flagBless.RemoteKey) + len(flagBless.RemoteToken)) > 0 {
					return fmt.Errorf("--remote-key and --remote-token should not be specified when --remote-arg-file is")
				}
			} else if len(args) > 2 {
				return fmt.Errorf("got %d arguments, require at most 2", len(args))
			} else if (len(flagBless.RemoteKey) == 0) != (len(flagBless.RemoteToken) == 0) {
				return fmt.Errorf("either both --remote-key and --remote-token should be set, or neither should")
			}
			p := v23.GetPrincipal(ctx)

			var (
				err  error
				with security.Blessings
			)
			if len(flagBless.With) > 0 {
				if with, err = decodeBlessings(flagBless.With); err != nil {
					return fmt.Errorf("failed to read blessings from --with=%q: %v", flagBless.With, err)
				}
			} else {
				with, _ = p.BlessingStore().Default()
			}
			caveats, err := caveatsFromFlags(flagBless.For, &flagBless.Caveat)
			if err != nil {
				return err
			}
			if len(caveats) == 0 {
				if flagBless.RequireCaveats {
					if err := confirmNoCaveats(env); err != nil {
						return err
					}
				}
				caveats = []security.Caveat{security.UnconstrainedUse()}
			}
			if len(caveats) == 0 {
				return errNoCaveats
			}

			tobless, extension, remoteKey, remoteToken, err := blessArgs(
				env,
				flagBless.RemoteKey,
				flagBless.RemoteToken,
				flagBless.RemoteArgFile,
				args,
			)
			if err != nil {
				return err
			}

			// Send blessings to a "server" started by a "recvblessings" command, either
			// with the --remote-arg-file flag, or with --remote-key and --remote-token flags.
			if len(remoteKey) > 0 {
				granter := &granter{with, extension, caveats, remoteKey}
				return blessOverNetwork(ctx, tobless, granter, remoteToken)
			}

			// Blessing a principal whose key is available locally.
			blessings, err := blessOverFileSystem(p, tobless, with, extension, caveats)
			if err != nil {
				return err
			}
			return dumpBlessings(blessings)
		}),
	}

	cmdGetPublicKey = &cmdline.Command{
		Name:  "publickey",
		Short: "Prints the public key of the principal.",
		Long: `
Prints out the public key of the principal specified by the environment
that this tool is running in.

The key is printed as a base64url encoded bytes (RFC 4648, Section 5) of the
DER-format representation of the key (suitable to be provided as an argument to
the 'recognize' command for example).

With --pretty, a 16-byte fingerprint of the key instead. This format is easier
for humans to read and is used in output of other commands in this program, but
is not suitable as an argument to the 'recognize' command.
`,
		FlagDefs: flagGetPublicKeyDef,
		Runner: v23cmd.RunnerFunc(func(ctx *context.T, env *cmdline.Env, args []string) error {
			key := v23.GetPrincipal(ctx).PublicKey()
			if flagGetPublicKey.Pretty {
				fmt.Println(key)
				return nil
			}
			der, err := key.MarshalBinary()
			if err != nil {
				return fmt.Errorf("corrupted key: %v", err)
			}
			fmt.Println(base64.URLEncoding.EncodeToString(der))
			return nil
		}),
	}

	cmdGetTrustedRoots = &cmdline.Command{
		Name:  "recognizedroots",
		Short: "Return recognized blessings, and their associated public key.",
		Long: `
Shows list of blessing names that the principal recognizes, and their associated
public key. If the principal is operating as a client, contacted servers must
appear on this list. If the principal is operating as a server, clients must
present blessings derived from this list.
`,
		Runner: v23cmd.RunnerFunc(func(ctx *context.T, env *cmdline.Env, args []string) error {
			fmt.Print(v23.GetPrincipal(ctx).Roots().DebugString())
			return nil
		}),
	}

	cmdGetPeerMap = &cmdline.Command{
		Name:  "peermap",
		Short: "Shows the map from peer pattern to which blessing name to present.",
		Long: `
Shows the map from peer pattern to which blessing name to present.
If the principal operates as a server, it presents its default blessing to all peers.
If the principal operates as a client, it presents the map value associated with
the peer it contacts.
`,
		Runner: v23cmd.RunnerFunc(func(ctx *context.T, env *cmdline.Env, args []string) error {
			fmt.Print(v23.GetPrincipal(ctx).BlessingStore().DebugString())
			return nil
		}),
	}

	cmdGetForPeer = &cmdline.Command{
		Name:  "forpeer",
		Short: "Return blessings marked for the provided peer",
		Long: `
Returns blessings that are marked for the provided peer in the
BlessingStore specified by the environment that this tool is
running in.
Providing --names will print the blessings' chain names.
Providing --rootkey <chain_name> will print the root key of the certificate chain
with chain_name.
Providing --caveats <chain_name> will print the caveats on the certificate chain
with chain_name.
`,
		ArgsName: "[<peer_1> ... <peer_k>]",
		ArgsLong: `
<peer_1> ... <peer_k> are the (human-readable string) blessings bound
to the peer. The returned blessings are marked with a pattern that is
matched by at least one of these. If no arguments are specified,
store.forpeer returns the blessings that are marked for all peers (i.e.,
blessings set on the store with the "..." pattern).
`,
		FlagDefs: flagGetForPeerDef,
		Runner: v23cmd.RunnerFunc(func(ctx *context.T, env *cmdline.Env, args []string) error {
			return printBlessingsInfo(
				flagGetForPeer.Names,
				flagGetForPeer.RootKey,
				flagGetForPeer.Caveats,
				v23.GetPrincipal(ctx).BlessingStore().ForPeer(args...))
		}),
	}

	cmdGetDefault = &cmdline.Command{
		Name:  "default",
		Short: "Return blessings marked as default",
		Long: `
Returns blessings that are marked as default in the BlessingStore specified by
the environment that this tool is running in.
Providing --names will print the default blessings' chain names.
Providing --rootkey <chain_name> will print the root key of the certificate chain
with chain_name.
Providing --caveats <chain_name> will print the caveats on the certificate chain
with chain_name.
`,
		FlagDefs: flagGetDefaultsDef,
		Runner: v23cmd.RunnerFunc(func(ctx *context.T, env *cmdline.Env, args []string) error {
			def, _ := v23.GetPrincipal(ctx).BlessingStore().Default()
			return printBlessingsInfo(
				flagGetDefaults.Names,
				flagGetDefaults.RootKey,
				flagGetDefaults.Caveats,
				def)
		}),
	}

	cmdSetForPeer = &cmdline.Command{
		Name:  "forpeer",
		Short: "Set provided blessings for peer",
		Long: `
Marks the provided blessings to be shared with the provided peers on the
BlessingStore specified by the environment that this tool is running in.

'set b pattern' marks the intention to reveal b to peers who
present blessings of their own matching 'pattern'.

'set nil pattern' can be used to remove the blessings previously
associated with the pattern (by a prior 'set' command).

It is an error to call 'set forpeer' with blessings whose public
key does not match the public key of this principal specified
by the environment.
`,
		ArgsName: "<file> <pattern>",
		ArgsLong: `
<file> is the path to a file containing a blessing typically obtained
from this tool. - is used for STDIN.

<pattern> is the BlessingPattern used to identify peers with whom this
blessing can be shared with.
`,
		FlagDefs: flagSetForPeerDef,
		Runner: v23cmd.RunnerFunc(func(ctx *context.T, env *cmdline.Env, args []string) error {
			if len(args) != 2 {
				return fmt.Errorf("requires exactly two arguments <file>, <pattern>, provided %d", len(args))
			}
			blessings, err := decodeBlessings(args[0])
			if err != nil {
				return fmt.Errorf("failed to decode provided blessings: %v", err)
			}
			pattern := security.BlessingPattern(args[1])
			p, err := getMutablePrincipal(root)
			if err != nil {
				return err
			}
			if _, err := p.BlessingStore().Set(blessings, pattern); err != nil {
				return fmt.Errorf("failed to set blessings %v for peers %v: %v", blessings, pattern, err)
			}
			if flagSetForPeer.AddToRoots {
				if err := security.AddToRoots(p, blessings); err != nil {
					return fmt.Errorf("AddToRoots failed: %v", err)
				}
			}
			return nil
		}),
	}

	cmdRecognize = &cmdline.Command{
		Name:  "recognize",
		Short: "Add to the set of identity providers recognized by this principal",
		Long: `
Adds an identity provider to the set of recognized root public keys for this principal.

It accepts either a single argument (which points to a file containing a blessing)
or two arguments (a name and a base64url-encoded DER-encoded public key).

For example, to make the principal in credentials directory A recognize the
root of the default blessing in credentials directory B:
  principal -v23.credentials=B bless A some_extension |
  principal -v23.credentials=A recognize -
The extension 'some_extension' has no effect in the command above.

Or to make the principal in credentials directory A recognize the public key
for the principal in credentials directory B for blessing pattern P:
  principal -v23.credentials=A recognize P $(principal -v23.credentials=B get publickey)
`,
		ArgsName: "<blessing pattern|blessing> [<key>]",
		ArgsLong: `
<blessing> is the path to a file containing a blessing typically obtained from
this tool. - is used for STDIN.

<blessing pattern> is the blessing pattern for which <key> should be recognized.

<key> is a base64url-encoded, DER-encoded public key, such as that printed by "principal get publickey".
`,
		Runner: v23cmd.RunnerFunc(func(ctx *context.T, env *cmdline.Env, args []string) error {
			if len(args) != 1 && len(args) != 2 {
				return fmt.Errorf("requires either one argument <file>, or two arguments <blessing pattern> <key>, provided %d", len(args))
			}
			p, err := getMutablePrincipal(root)
			if err != nil {
				return err
			}
			if len(args) == 1 {
				blessings, err := decodeBlessings(args[0])
				if err != nil {
					return fmt.Errorf("failed to decode provided blessings: %v", err)
				}
				if err := security.AddToRoots(p, blessings); err != nil {
					return fmt.Errorf("AddToRoots failed: %v", err)
				}
				return nil
			}
			// len(args) == 2
			der, err := base64.URLEncoding.DecodeString(args[1])
			if err != nil {
				return fmt.Errorf("invalid base64url encoding of public key: %v", err)
			}
			return p.Roots().Add(der, security.BlessingPattern(args[0]))
		}),
	}
	cmdUnion = &cmdline.Command{
		Name:  "union",
		Short: "Merge multiple blessings into one",
		Long: `
Merges multiple blessings into one.

It accepts multiple base64url-encoded blessings. Each argument can be a file
containing a blessing, or the blessing itself. It returns the union of all the
blessings.

For example, to merge the blessings contained in files A and B:
  principal union A B, or
  principal union $(cat A) $(cat B)
`,
		ArgsName: "[<blessing> | <blessing file>...]",
		ArgsLong: `
<blessing> is a base64url-encoded blessing.

<blessing file> is a file that contains a base64url-encoded blessing.
`,
		Runner: v23cmd.RunnerFunc(func(ctx *context.T, env *cmdline.Env, args []string) error {
			var ret security.Blessings
			for _, b := range args {
				encoded := b
				if b, err := ioutil.ReadFile(b); err == nil {
					encoded = string(b)
				}
				var blessings security.Blessings
				if err := base64urlVomDecode(encoded, &blessings); err != nil {
					return err
				}
				var err error
				if ret, err = security.UnionOfBlessings(ret, blessings); err != nil {
					return err
				}
			}
			out, err := base64urlVomEncode(ret)
			if err != nil {
				return err
			}
			fmt.Fprintln(env.Stdout, out)
			return nil
		}),
	}

	cmdSetDefault = &cmdline.Command{
		Name:  "default",
		Short: "Set provided blessings as default",
		Long: `
Sets the provided blessings as default in the BlessingStore specified by the
environment that this tool is running in.

It is an error to call 'set default' with blessings whose public key does
not match the public key of the principal specified by the environment.
`,
		ArgsName: "<file>",
		ArgsLong: `
<file> is the path to a file containing a blessing typically obtained from
this tool. - is used for STDIN.
`,
		FlagDefs: flagSetDefaultsDef,
		Runner: v23cmd.RunnerFunc(func(ctx *context.T, env *cmdline.Env, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("requires exactly one argument, <file>, provided %d", len(args))
			}
			blessings, err := decodeBlessings(args[0])
			if err != nil {
				return fmt.Errorf("failed to decode provided blessings: %v", err)
			}
			p, err := getMutablePrincipal(root)
			if err != nil {
				return err
			}
			if err := p.BlessingStore().SetDefault(blessings); err != nil {
				return fmt.Errorf("failed to set blessings %v as default: %v", blessings, err)
			}
			if flagSetDefaults.AddToRoots {
				if err := security.AddToRoots(p, blessings); err != nil {
					return fmt.Errorf("AddToRoots failed: %v", err)
				}
			}
			return nil
		}),
	}

	cmdCreate = &cmdline.Command{
		Name:  "create",
		Short: "Create a new principal and persist it into a directory",
		Long: `
Creates a new principal with a single optional self-blessed blessing and writes
it out to the provided directory. The same directory can then be used to set the
V23_CREDENTIALS environment variable for other vanadium applications.

The operation fails if the directory already contains a principal. In this case
the --overwrite flag can be provided to clear the directory and write out the
new principal.
`,
		ArgsName: "<directory> [<blessing>]",
		ArgsLong: `
<directory> is the directory to which the new principal will be persisted.

<blessing> is the optional self-blessed blessing that the principal will be
setup to use by default.  If a blessing argument is not provided, the new
principal will have no blessings.
	`,
		FlagDefs: flagCreateDef,
		Runner: cmdline.RunnerFunc(func(env *cmdline.Env, args []string) error {
			if len(args) < 1 || len(args) > 2 {
				return fmt.Errorf("requires one or two arguments: <directory> [and optional <blessing>], provided %d", len(args))
			}
			dir := args[0]
			var pass []byte
			if flagCreate.WithPassphrase {
				var err error
				if pass, err = passphrase.Get("Enter passphrase (entering nothing will store the principal key unencrypted): "); err != nil {
					return err
				}
				defer vsecurity.ZeroPassphrase(pass)
			}
			p, err := createPersistentPrincipal(
				gocontext.TODO(),
				dir,
				flagCreate.KeyType,
				flagCreate.SSHAgentPublicKeyFile,
				pass,
				flagCreate.CreateOverwrite)
			if err != nil {
				return fmt.Errorf("failed to create principal: %v", err)
			}
			if len(args) == 2 {
				name := args[1]
				blessings, err := p.BlessSelf(name)
				if err != nil {
					return fmt.Errorf("BlessSelf(%q) failed: %v", name, err)
				}
				if err := vsecurity.SetDefaultBlessings(p, blessings); err != nil {
					return fmt.Errorf("could not set blessings %v as default: %v", blessings, err)
				}
			}
			return nil
		}),
	}

	cmdFork = &cmdline.Command{
		Name:  "fork",
		Short: "Fork a new principal from the principal that this tool is running as and persist it into a directory",
		Long: `
Creates a new principal with a blessing from the principal specified by the
environment that this tool is running in, and writes it out to the provided
directory. The blessing that will be extended is the default one from the
blesser's store, or specified by the --with flag. Expiration on the blessing
are controlled via the --for flag. Additional caveats on the blessing are
controlled with the --caveat flag. The blessing is marked as default and
shareable with all peers on the new principal's blessing store.

The operation fails if the directory already contains a principal. In this case
the --overwrite flag can be provided to clear the directory and write out the
forked principal.
`,
		ArgsName: "<directory> <extension>",
		ArgsLong: `
<directory> is the directory to which the forked principal will be persisted.

<extension> is the extension under which the forked principal is blessed.
	`,
		FlagDefs: flagForkDef,
		Runner: v23cmd.RunnerFunc(func(ctx *context.T, env *cmdline.Env, args []string) error {
			if len(args) != 2 {
				return fmt.Errorf("requires exactly two arguments: <directory> and <extension>, provided %d", len(args))
			}
			dir, extension := args[0], args[1]
			caveats, err := caveatsFromFlags(flagFork.For, &flagFork.Caveat)
			if err != nil {
				return err
			}
			if !flagFork.RequireCaveats && len(caveats) == 0 {
				caveats = []security.Caveat{security.UnconstrainedUse()}
			}
			if len(caveats) == 0 {
				return errNoCaveats
			}
			var with security.Blessings
			if len(flagFork.With) > 0 {
				if with, err = decodeBlessings(flagFork.With); err != nil {
					return fmt.Errorf("failed to read blessings from --with=%q: %v", flagFork.With, err)
				}
			} else {
				with, _ = v23.GetPrincipal(ctx).BlessingStore().Default()
			}

			if flagFork.CreateOverwrite {
				if err := os.RemoveAll(dir); err != nil {
					return err
				}
			}
			var pass []byte
			if flagFork.WithPassphrase {
				var err error
				if pass, err = passphrase.Get("Enter passphrase (entering nothing will store the principal key unencrypted): "); err != nil {
					return err
				}
			}
			p, err := createPersistentPrincipal(
				gocontext.TODO(),
				dir,
				flagFork.KeyType,
				flagFork.SSHAgentPublicKeyFile,
				pass,
				flagFork.CreateOverwrite)
			if err != nil {
				return fmt.Errorf("failed to create principal: %v", err)
			}
			key := p.PublicKey()
			rp := v23.GetPrincipal(ctx)
			blessings, err := rp.Bless(key, with, extension, caveats[0], caveats[1:]...)
			if err != nil {
				return fmt.Errorf("Bless(%v, %v, %q, ...) failed: %v", key, with, extension, err)
			}
			if err := vsecurity.SetDefaultBlessings(p, blessings); err != nil {
				return fmt.Errorf("could not set blessings %v as default: %v", blessings, err)
			}
			return nil
		}),
	}

	cmdSeekBlessings = &cmdline.Command{
		Name:  "seekblessings",
		Short: "Seek blessings from a web-based Vanadium blessing service",
		Long: `
Seeks blessings from a web-based Vanadium blesser which
requires the caller to first authenticate with Google using OAuth. Simply
run the command to see what happens.

The blessings are sought for the principal specified by the environment that
this tool is running in.

The blessings obtained are set as default unless the --set-default flag is
set to false, and are also set for sharing with all peers unless a more
specific peer pattern is provided using the --for-peer flag.
`,
		FlagDefs: flagSeekBlessingsDef,

		Runner: v23cmd.RunnerFunc(func(ctx *context.T, env *cmdline.Env, args []string) error {
			p := v23.GetPrincipal(ctx)

			blessedChan := make(chan string)
			defer close(blessedChan)
			macaroonChan, err := getMacaroonForBlessRPC(p.PublicKey(), flagSeekBlessings.From, blessedChan, flagSeekBlessings.Browser)
			if err != nil {
				return fmt.Errorf("failed to get macaroon from Vanadium blesser: %v", err)
			}

			blessings, err := exchangeMacaroonForBlessing(ctx, macaroonChan)
			if err != nil {
				return err
			}
			blessedChan <- fmt.Sprint(blessings)
			// Wait for getTokenForBlessRPC to clean up:
			<-macaroonChan

			if flagSeekBlessings.SetDefault {
				if err := p.BlessingStore().SetDefault(blessings); err != nil {
					return fmt.Errorf("failed to set blessings %v as default: %v", blessings, err)
				}
			}
			if pattern := security.BlessingPattern(flagSeekBlessings.ForPeer); len(pattern) > 0 {
				if _, err := p.BlessingStore().Set(blessings, pattern); err != nil {
					return fmt.Errorf("failed to set blessings %v for peers %v: %v", blessings, pattern, err)
				}
			}
			if flagSeekBlessings.AddToRoots {
				if err := security.AddToRoots(p, blessings); err != nil {
					return fmt.Errorf("AddToRoots failed: %v", err)
				}
			}
			fmt.Fprintf(env.Stdout, "Received blessings: %v\n", blessings)
			return nil
		}),
	}

	cmdRecvBlessings = &cmdline.Command{
		Name:  "recvblessings",
		Short: "Receive blessings sent by another principal and use them as the default",
		Long: `
Allow another principal (likely a remote process) to bless this one.

This command sets up the invoker (this process) to wait for a blessing
from another invocation of this tool (remote process) and prints out the
command to be run as the remote principal.

The received blessings are set as default unless the --set-default flag is
set to false, and are also set for sharing with all peers unless a more
specific peer pattern is provided using the --for-peer flag.

TODO(ashankar,cnicolaou): Make this next paragraph possible! Requires
the ability to obtain the proxied endpoint.

Typically, this command should require no arguments.
However, if the sender and receiver are on different network domains, it may
make sense to use the --v23.proxy flag:
    principal --v23.proxy=proxy recvblessings

The command to be run at the sender is of the form:
    principal bless --remote-key=KEY --remote-token=TOKEN ADDRESS EXTENSION

The --remote-key flag is used to by the sender to "authenticate" the receiver,
ensuring it blesses the intended recipient and not any attacker that may have
taken over the address.

The --remote-token flag is used by the sender to authenticate itself to the
receiver. This helps ensure that the receiver rejects blessings from senders
who just happened to guess the network address of the 'recvblessings'
invocation.

If the --remote-arg-file flag is provided to recvblessings, the remote key, remote token
and object address of this principal will be written to the specified location.
This file can be supplied to bless:
    principal bless --remote-arg-file FILE EXTENSION

`,
		FlagDefs: flagRecvBlessingsDef,
		Runner: v23cmd.RunnerFunc(func(ctx *context.T, env *cmdline.Env, args []string) error {
			if len(args) != 0 {
				return fmt.Errorf("command accepts no arguments")
			}
			var token [24]byte
			if _, err := rand.Read(token[:]); err != nil {
				return fmt.Errorf("unable to generate token: %v", err)
			}
			p, err := getMutablePrincipal(root)
			if err != nil {
				return err
			}
			service := &recvBlessingsService{
				setDefault:           flagRecvBlessings.SetDefault,
				recvBlessingsForPeer: flagRecvBlessings.ForPeer,
				addToRoots:           true,
				principal:            p,
				token:                base64.URLEncoding.EncodeToString(token[:]),
				notify:               make(chan error),
			}
			_, server, err := v23.WithNewServer(ctx, "", service, security.AllowEveryone())
			if err != nil {
				return fmt.Errorf("failed to create server to listen for blessings: %v", err)
			}
			name := server.Status().Endpoints[0].Name()
			fmt.Println("Run the following command on behalf of the principal that will send blessings:")
			fmt.Println("You may want to adjust flags affecting the caveats on this blessing, for example using")
			fmt.Println("the --for flag")
			fmt.Println()
			if len(flagRecvBlessings.RemoteArgFile) > 0 {
				if err := writeRecvBlessingsInfo(flagRecvBlessings.RemoteArgFile, p.PublicKey().String(), service.token, name); err != nil {
					return fmt.Errorf("failed to write recvblessings info to %v: %v", flagRecvBlessings.RemoteArgFile, err)
				}
				fmt.Printf("make %q accessible to the blesser, possibly by copying the file over and then run:\n", flagRecvBlessings.RemoteArgFile)
				fmt.Printf("principal bless --remote-arg-file=%v", flagRecvBlessings.RemoteArgFile)
			} else {
				fmt.Printf("principal bless --remote-key=%v --remote-token=%v %v\n", p.PublicKey(), service.token, name)
			}
			fmt.Println()
			fmt.Println("...waiting for sender..")
			return <-service.notify
		}),
	}

	root = &cmdline.Command{
		Name:  "principal",
		Short: "creates and manages Vanadium principals and blessings",
		Long: `
Command principal creates and manages Vanadium principals and blessings.

All objects are printed using base64url-vom-encoding.
`,
	}
)

func printAnnotatedBlessingsNames(b security.Blessings) string {
	// If the Blessings are expired, print a message saying so.
	expiredMessage := ""
	if exp := b.Expiry(); !exp.IsZero() && exp.Before(time.Now()) {
		expiredMessage = " [EXPIRED]"
	}
	return fmt.Sprintf("%v%s", b, expiredMessage)
}

func blessArgs(env *cmdline.Env, remoteArgKey, remoteArgToken, remoteArgFile string, args []string) (tobless, extension, remoteKey, remoteToken string, err error) {
	extensionInArgs := false
	if len(remoteArgFile) == 0 {
		if len(args) == 0 {
			err = fmt.Errorf("no remote-arg-file flag and no arguments")
			return
		}
		tobless = args[0]
		remoteKey = remoteArgKey
		remoteToken = remoteArgToken
		extensionInArgs = len(args) > 1
	} else if len(remoteArgFile) > 0 {
		remoteKey, remoteToken, tobless, err = blessArgsFromFile(remoteArgFile)
		extensionInArgs = len(args) > 0
	}
	if extensionInArgs {
		extension = args[len(args)-1]
	} else {
		extension, err = readFromStdin(env, "Extension to use for blessing:")
	}
	return
}

func confirmNoCaveats(env *cmdline.Env) error {
	text, err := readFromStdin(env, `WARNING: No caveats provided
It is generally dangerous to bless another principal without any caveats as
that gives them unrestricted access to the blesser's credentials.

Caveats can be specified with the --for or --caveat flags.

Do you really wish to bless without caveats? (YES to confirm)`)
	if err != nil || strings.ToUpper(text) != "YES" {
		return errNoCaveats
	}
	return nil
}

func readFromStdin(env *cmdline.Env, prompt string) (string, error) {
	fmt.Fprintf(env.Stdout, "%v ", prompt)
	os.Stdout.Sync()
	// Cannot use bufio because that may "lose" data beyond the line (the
	// remainder in the buffer).
	// Do the inefficient byte-by-byte scan for now - shouldn't be a problem
	// given the common use case. If that becomes a problem, switch to bufio
	// and share the bufio.Reader between multiple calls to readFromStdin.
	buf := make([]byte, 0, 100)
	r := make([]byte, 1)
	for {
		n, err := env.Stdin.Read(r)
		if n == 1 && r[0] == '\n' {
			break
		}
		if n == 1 {
			buf = append(buf, r[0])
			continue
		}
		if err != nil {
			return "", err
		}
	}
	return strings.TrimSpace(string(buf)), nil
}

func blessOverFileSystem(p security.Principal, tobless string, with security.Blessings, extension string, caveats []security.Caveat) (security.Blessings, error) {
	var key security.PublicKey
	if finfo, err := os.Stat(tobless); err == nil && finfo.IsDir() {
		other, err := vsecurity.LoadPersistentPrincipal(tobless, nil)
		if err != nil {

			return security.Blessings{}, fmt.Errorf("failed to read principal in directory %q: %v", tobless, err)
		}
		key = other.PublicKey()
	} else if str, err := read(tobless); err != nil {
		return security.Blessings{}, fmt.Errorf("failed to read %q: %v", tobless, err)
	} else if b64, err := base64.URLEncoding.DecodeString(str); err != nil {
		return security.Blessings{}, fmt.Errorf("failed to decode base64url encoded bytes in %q: %v", tobless, err)
	} else if key, err = security.UnmarshalPublicKey(b64); err != nil {
		// Not a public key, maybe a blessings object?
		var b security.Blessings
		if errb := vom.Decode(b64, &b); errb != nil {
			return security.Blessings{}, fmt.Errorf("failed to decode blessings (%v) or public key (%v) from %q", errb, err, tobless)
		}
		key = b.PublicKey()
	}
	return p.Bless(key, with, extension, caveats[0], caveats[1:]...)
}

type recvBlessingsInfo struct {
	RemoteKey   string `json:"remote_key"`
	RemoteToken string `json:"remote_token"`
	Name        string `json:"name"`
}

func writeRecvBlessingsInfo(fname string, remoteKey, remoteToken, name string) error {
	f, err := os.OpenFile(fname, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	b, err := json.Marshal(recvBlessingsInfo{remoteKey, remoteToken, name})
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		return err
	}
	return nil
}

func blessArgsFromFile(fname string) (remoteKey, remoteToken, tobless string, err error) {
	blessJSON, err := ioutil.ReadFile(fname)
	if err != nil {
		return "", "", "", err
	}
	var binfo recvBlessingsInfo
	if err := json.Unmarshal(blessJSON, &binfo); err != nil {
		return "", "", "", err
	}
	return binfo.RemoteKey, binfo.RemoteToken, binfo.Name, err
}

func main() {
	cmdline.HideGlobalFlagsExcept()

	cmdSet := &cmdline.Command{
		Name:  "set",
		Short: "Mutate the principal's blessings.",
		Long: `
Commands to mutate the blessings of the principal.

All input blessings are expected to be serialized using base64url-vom-encoding.
See 'principal get'.
`,
		Children: []*cmdline.Command{cmdSetDefault, cmdSetForPeer},
	}

	cmdGet := &cmdline.Command{
		Name:  "get",
		Short: "Read the principal's blessings.",
		Long: `
Commands to inspect the blessings of the principal.

All blessings are printed to stdout using base64url-vom-encoding.
`,
		Children: []*cmdline.Command{cmdGetDefault, cmdGetForPeer, cmdGetPublicKey, cmdGetTrustedRoots, cmdGetPeerMap},
	}

	root.Children = []*cmdline.Command{cmdCreate, cmdFork, cmdSeekBlessings, cmdRecvBlessings, cmdDump, cmdDumpBlessings, cmdDumpRoots, cmdBlessSelf, cmdBless, cmdSet, cmdGet, cmdRecognize, cmdUnion}
	cmdline.Main(root)
}

func decodeBlessings(fname string) (security.Blessings, error) {
	var b security.Blessings
	err := decode(fname, &b)
	return b, err
}

func dumpBlessings(blessings security.Blessings) error {
	if blessings.IsZero() {
		return fmt.Errorf("no blessings found")
	}
	str, err := base64urlVomEncode(blessings)
	if err != nil {
		return fmt.Errorf("base64url-vom encoding failed: %v", err)
	}
	fmt.Println(str)
	return nil
}

func printBlessingsInfo(names bool, rootKey, caveats string, blessings security.Blessings) error {
	if blessings.IsZero() {
		return fmt.Errorf("no blessings found")
	}
	switch {
	case names:
		fmt.Println(strings.ReplaceAll(fmt.Sprint(blessings), ",", "\n"))
		return nil
	case len(rootKey) > 0:
		chain, err := getChainByName(blessings, rootKey)
		if err != nil {
			return err
		}
		fmt.Println(rootkey(chain))
		return nil
	case len(caveats) > 0:
		chain, err := getChainByName(blessings, caveats)
		if err != nil {
			return err
		}
		cavs, err := prettyPrintCaveats(chain)
		if err != nil {
			return err
		}
		for _, c := range cavs {
			fmt.Println(c)
		}
		return nil
	}
	return dumpBlessings(blessings)
}

func prettyPrintCaveats(chain []security.Certificate) ([]string, error) {
	var cavs []security.Caveat
	for _, cert := range chain {
		cavs = append(cavs, cert.Caveats...)
	}
	var s []string
	for _, cav := range cavs {
		if cav.Id == security.PublicKeyThirdPartyCaveat.Id {
			c := cav.ThirdPartyDetails()
			s = append(s, fmt.Sprintf("ThirdPartyCaveat: Requires discharge from %v (ID=%q)", c.Location(), c.ID()))
			continue
		}
		var param interface{}
		if err := vom.Decode(cav.ParamVom, &param); err != nil {
			return nil, err
		}
		switch cav.Id {
		case security.ConstCaveat.Id:
			// In the case a ConstCaveat is specified, we only want to print it
			// if it never validates.
			if !param.(bool) {
				s = append(s, "Never validates")
			}
		case security.ExpiryCaveat.Id:
			s = append(s, fmt.Sprintf("Expires at %v", param))
		case security.MethodCaveat.Id:
			s = append(s, fmt.Sprintf("Restricted to methods %v", param))
		case security.PeerBlessingsCaveat.Id:
			s = append(s, fmt.Sprintf("Restricted to peers with blessings %v", param))
		default:
			s = append(s, cav.String())
		}
	}
	return s, nil
}

func getChainByName(b security.Blessings, name string) ([]security.Certificate, error) {
	wire, err := blessings2wire(b)
	if err != nil {
		return nil, err
	}
	for _, chain := range wire.CertificateChains {
		if chainName(chain) == name {
			return chain, nil
		}
	}
	return nil, fmt.Errorf("no chains of name %v in %v", name, b)
}

func read(fname string) (string, error) {
	if len(fname) == 0 {
		return "", nil
	}
	f := os.Stdin
	if fname != "-" {
		var err error
		if f, err = os.Open(fname); err != nil {
			return "", fmt.Errorf("failed to open %q: %v", fname, err)
		}
	}
	defer f.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, f); err != nil {
		return "", fmt.Errorf("failed to read %q: %v", fname, err)
	}
	return buf.String(), nil
}

func decode(fname string, val interface{}) error {
	str, err := read(fname)
	if err != nil {
		return err
	}
	if err := base64urlVomDecode(str, val); err != nil || val == nil {
		return fmt.Errorf("failed to decode %q: %v", fname, err)
	}
	return nil
}

func defaultBlessingName() string {
	var name string
	if user, _ := user.Current(); user != nil && len(user.Username) > 0 {
		name = user.Username
	} else {
		name = "anonymous"
	}
	if host, _ := os.Hostname(); len(host) > 0 {
		name = name + "@" + host
	}
	return name
}

func rootkey(chain []security.Certificate) string {
	if len(chain) == 0 {
		return "<empty certificate chain>"
	}
	key, err := security.UnmarshalPublicKey(chain[0].PublicKey)
	if err != nil {
		return fmt.Sprintf("<invalid PublicKey: %v>", err)
	}
	return fmt.Sprintf("%v", key)
}

func chainName(chain []security.Certificate) string {
	exts := make([]string, len(chain))
	for i, cert := range chain {
		exts[i] = cert.Extension
	}
	return strings.Join(exts, security.ChainSeparator)
}

func base64urlVomEncode(i interface{}) (string, error) {
	buf := &bytes.Buffer{}
	closer := base64.NewEncoder(base64.URLEncoding, buf)
	enc := vom.NewEncoder(closer)
	if err := enc.Encode(i); err != nil {
		return "", err
	}
	// Must close the base64 encoder to flush out any partially written
	// blocks.
	if err := closer.Close(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func base64urlVomDecode(s string, i interface{}) error {
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return err
	}
	dec := vom.NewDecoder(bytes.NewBuffer(b))
	return dec.Decode(i)
}

type recvBlessingsService struct {
	setDefault           bool
	recvBlessingsForPeer string
	addToRoots           bool
	principal            security.Principal
	notify               chan error
	token                string
}

func (r *recvBlessingsService) Grant(_ *context.T, call rpc.ServerCall, token string) error {
	b := call.GrantedBlessings()
	if b.IsZero() {
		return fmt.Errorf("no blessings granted by sender")
	}
	if len(token) != len(r.token) {
		// A timing attack can be used to figure out the length
		// of the token, but then again, so can looking at the
		// source code. So, it's okay.
		return fmt.Errorf("blessings received from unexpected sender")
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(r.token)) != 1 {
		return fmt.Errorf("blessings received from unexpected sender")
	}
	if r.setDefault {
		if err := r.principal.BlessingStore().SetDefault(b); err != nil {
			return fmt.Errorf("failed to set blessings %v as default: %v", b, err)
		}
	}
	if pattern := security.BlessingPattern(r.recvBlessingsForPeer); len(pattern) > 0 {
		if _, err := r.principal.BlessingStore().Set(b, pattern); err != nil {
			return fmt.Errorf("failed to set blessings %v for peers %v: %v", b, pattern, err)
		}
	}
	if r.addToRoots {
		if err := security.AddToRoots(r.principal, b); err != nil {
			return fmt.Errorf("failed to add blessings to recognized roots: %v", err)
		}
	}
	fmt.Println("Received blessings:", b)
	r.notify <- nil
	return nil
}

type granter struct {
	with      security.Blessings
	extension string
	caveats   []security.Caveat
	serverKey string
}

func (g *granter) Grant(ctx *context.T, call security.Call) (security.Blessings, error) {
	server := call.RemoteBlessings()
	p := call.LocalPrincipal()
	if got := fmt.Sprintf("%v", server.PublicKey()); got != g.serverKey {
		// If the granter returns an error, the RPC framework should
		// abort the RPC before sending the request to the server.
		// Thus, there is no concern about leaking the token to an
		// imposter server.
		return security.Blessings{}, fmt.Errorf("key mismatch: Remote end has public key %v, want %v", got, g.serverKey)
	}
	return p.Bless(server.PublicKey(), g.with, g.extension, g.caveats[0], g.caveats[1:]...)
}
func (*granter) RPCCallOpt() {}

func blessOverNetwork(ctx *context.T, object string, granter *granter, remoteToken string) error {
	client := v23.GetClient(ctx)
	// The receiver is being authorized based on the hash of its public key
	// (see Grant), so it should be fine to ignore the blessing names in the endpoint
	// (which are likely to not be recognized by the sender anyway).
	//
	// At worst, there is a privacy leak of the senders intent to send some
	// blessings.  That could be addressed by making the full public key of
	// the recipeint available to the sender and using
	// options.SecurityAuthorizer{security.PublicKeyAuthorizer()} instead
	// of providing a "hash" of the recipients public key and verifying in
	// the Granter implementation.
	if err := client.Call(
		ctx,
		object,
		"Grant",
		[]interface{}{remoteToken},
		nil,
		granter,
		options.ServerAuthorizer{Authorizer: security.AllowEveryone()},
		options.NameResolutionAuthorizer{Authorizer: security.AllowEveryone()}); err != nil {
		return fmt.Errorf("failed to make RPC to %q: %v", object, err)
	}
	return nil
}

func caveatsFromFlags(expiry time.Duration, caveatsFlag *caveatsFlag) ([]security.Caveat, error) {
	caveats, err := caveatsFlag.Compile()
	if err != nil {
		return nil, fmt.Errorf("failed to parse caveats: %v", err)
	}
	if expiry != 0 {
		ecav, err := security.NewExpiryCaveat(time.Now().Add(expiry))
		if err != nil {
			return nil, fmt.Errorf("failed to create expiration caveat: %v", err)
		}
		caveats = append(caveats, ecav)
	}
	return caveats, nil
}

// Circuitous route to get to the certificate chains.
// See comments on why security.MarshalBlessings is discouraged.
// Though, a better alternative is worth looking into.
func blessings2wire(b security.Blessings) (security.WireBlessings, error) {
	var wire security.WireBlessings
	data, err := vom.Encode(b)
	if err != nil {
		return wire, err
	}
	err = vom.Decode(data, &wire)
	return wire, err
}

func getMutablePrincipal(root *cmdline.Command) (security.Principal, error) {
	flagName := "v23.credentials"
	credFlag := root.ParsedFlags.Lookup(flagName)
	if credFlag == nil {
		return nil, fmt.Errorf("failed to lookup %v flag", flagName)
	}
	return vsecurity.LoadPersistentPrincipalWithPassphrasePrompt(credFlag.Value.String())
}

func createPersistentPrincipal(ctx gocontext.Context, dir, keyType, sshKey string, pass []byte, overwrite bool) (security.Principal, error) {
	removeExisting := func() error {
		if !overwrite {
			return nil
		}
		return os.RemoveAll(dir)
	}
	var privateKey interface{}
	var err error
	if len(sshKey) == 0 {
		privateKey, err = vsecurity.NewPrivateKey(keyType)
	} else {
		service := sshagent.NewClient()
		privateKey = vsecurity.SSHAgentHostedKey{
			PublicKeyFile: sshKey,
			Agent:         service,
		}
	}
	if err != nil {
		return nil, err
	}
	if err := removeExisting(); err != nil {
		return nil, err
	}
	return vsecurity.CreatePersistentPrincipalUsingKey(ctx, privateKey, dir, pass)
}

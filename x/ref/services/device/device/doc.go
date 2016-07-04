// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
Command device facilitates interaction with the Vanadium device manager.

Usage:
   device [flags] <command>

The device commands are:
   install       Install the given application.
   install-local Install the given application from the local system.
   uninstall     Uninstall the given application installation.
   associate     Tool for creating associations between Vanadium blessings and a
                 system account
   describe      Describe the device.
   claim         Claim the device.
   instantiate   Create an instance of the given application.
   delete        Delete the given application instance.
   run           Run the given application instance.
   kill          Kill the given application instance.
   revert        Revert the device manager or applications.
   update        Update the device manager or applications.
   status        Get device manager or application status.
   debug         Debug the device.
   acl           Tool for setting device manager Permissions
   publish       Publish the given application(s).
   ls            List applications.
   help          Display help for commands or topics

The global flags are:
 -v23.namespace.root=[/(dev.v.io:r:vprod:service:mounttabled)@ns.dev.v.io:8101]
   local namespace root; can be repeated to provided multiple roots
 -v23.proxy=
   object name of proxy service to use to export services across network
   boundaries

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
 -v23.permissions.file=map[]
   specify a perms file as <name>:<permsfile>
 -v23.permissions.literal=
   explicitly specify the runtime perms as a JSON-encoded access.Permissions.
   Overrides all --v23.permissions.file flags.
 -v23.tcp.address=
   address to listen on
 -v23.tcp.protocol=wsh
   protocol to listen with
 -v23.vtrace.cache-size=1024
   The number of vtrace traces to store in memory.
 -v23.vtrace.collect-regexp=
   Spans and annotations that match this regular expression will trigger trace
   collection.
 -v23.vtrace.dump-on-shutdown=true
   If true, dump all stored traces on runtime shutdown.
 -v23.vtrace.sample-rate=0
   Rate (from 0.0 to 1.0) to sample vtrace traces.
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

Device install

Install the given application and print the name of the new installation.

Usage:
   device install [flags] <device> <application>

<device> is the vanadium object name of the device manager's app service.

<application> is the vanadium object name of the application.

The device install flags are:
 -config={}
   JSON-encoded device.Config object, of the form:
   '{"flag1":"value1","flag2":"value2"}'
 -packages={}
   JSON-encoded application.Packages object, of the form:
   '{"pkg1":{"File":"object name 1"},"pkg2":{"File":"object name 2"}}'

Device install-local

Install the given application specified using a local path, and print the name
of the new installation.

Usage:
   device install-local [flags] <device> <title> [ENV=VAL ...] binary [--flag=val ...] [PACKAGES path ...]

<device> is the vanadium object name of the device manager's app service.

<title> is the app title.

This is followed by an arbitrary number of environment variable settings, the
local path for the binary to install, and arbitrary flag settings and args.
Optionally, this can be followed by 'PACKAGES' and a list of local files and
directories to be installed as packages for the app

The device install-local flags are:
 -config={}
   JSON-encoded device.Config object, of the form:
   '{"flag1":"value1","flag2":"value2"}'
 -packages={}
   JSON-encoded application.Packages object, of the form:
   '{"pkg1":{"File":"local file path1"},"pkg2":{"File":"local file path 2"}}'

Device uninstall

Uninstall the given application installation.

Usage:
   device uninstall [flags] <installation>

<installation> is the vanadium object name of the application installation to
uninstall.

Device associate - Tool for creating associations between Vanadium blessings and a system account

The associate tool facilitates managing blessing to system account associations.

Usage:
   device associate [flags] <command>

The device associate commands are:
   list        Lists the account associations.
   add         Add the listed blessings with the specified system account.
   remove      Removes system accounts associated with the listed blessings.

Device associate list

Lists all account associations.

Usage:
   device associate list [flags] <devicemanager>.

<devicemanager> is the name of the device manager to connect to.

Device associate add

Add the listed blessings with the specified system account.

Usage:
   device associate add [flags] <devicemanager> <systemName> <blessing>...

<devicemanager> is the name of the device manager to connect to. <systemName> is
the name of an account holder on the local system. <blessing>.. are the
blessings to associate systemAccount with.

Device associate remove

Removes system accounts associated with the listed blessings.

Usage:
   device associate remove [flags] <devicemanager>  <blessing>...

<devicemanager> is the name of the device manager to connect to. <blessing>...
is a list of blessings.

Device describe

Describe the device.

Usage:
   device describe [flags] <device>

<device> is the vanadium object name of the device manager's device service.

Device claim

Claim the device.

Usage:
   device claim [flags] <device> <grant extension> <pairing token> <device publickey>

<device> is the vanadium object name of the device manager's device service.

<grant extension> is used to extend the default blessing of the current
principal when blessing the app instance.

<pairing token> is a token that the device manager expects to be replayed during
a claim operation on the device.

<device publickey> is the marshalled public key of the device manager we are
claiming.

Device instantiate

Create an instance of the given application, provide it with a blessing, and
print the name of the new instance.

Usage:
   device instantiate [flags] <application installation> <grant extension>

<application installation> is the vanadium object name of the application
installation from which to create an instance.

<grant extension> is used to extend the default blessing of the current
principal when blessing the app instance.

Device delete

Delete the given application instance.

Usage:
   device delete [flags] <app instance>

<app instance> is the vanadium object name of the application instance to
delete.

Device run

Run the given application instance.

Usage:
   device run [flags] <app instance>

<app instance> is the vanadium object name of the application instance to run.

Device kill

Kill the given application instance.

Usage:
   device kill [flags] <app instance>

<app instance> is the vanadium object name of the application instance to kill.

Device revert

Revert the device manager or application instances and installations to a
previous version of their current version

Usage:
   device revert [flags] <name patterns...>

<name patterns...> are vanadium object names or glob name patterns corresponding
to the device manager service, or to application installations and instances.

The device revert flags are:
 -installation-state=!Uninstalled
   If non-empty, specifies allowed installation states (all others installations
   get filtered out). The value of the flag is a comma-separated list of values
   from among: [Active Uninstalled]. If the value is prefixed by '!', the list
   acts as a blacklist (all matching installations get filtered out).
 -instance-state=!Deleted
   If non-empty, specifies allowed instance states (all other instances get
   filtered out). The value of the flag is a comma-separated list of values from
   among: [Launching Running Dying NotRunning Updating Deleted]. If the value is
   prefixed by '!', the list acts as a blacklist (all matching instances get
   filtered out).
 -only-installations=false
   If set, only consider installations.
 -only-instances=false
   If set, only consider instances.
 -parallelism=BYKIND
   Specifies the level of parallelism for the handler execution. One of: [BYKIND
   FULL NONE].

Device update

Update the device manager or application instances and installations

Usage:
   device update [flags] <name patterns...>

<name patterns...> are vanadium object names or glob name patterns corresponding
to the device manager service, or to application installations and instances.

The device update flags are:
 -installation-state=!Uninstalled
   If non-empty, specifies allowed installation states (all others installations
   get filtered out). The value of the flag is a comma-separated list of values
   from among: [Active Uninstalled]. If the value is prefixed by '!', the list
   acts as a blacklist (all matching installations get filtered out).
 -instance-state=!Deleted
   If non-empty, specifies allowed instance states (all other instances get
   filtered out). The value of the flag is a comma-separated list of values from
   among: [Launching Running Dying NotRunning Updating Deleted]. If the value is
   prefixed by '!', the list acts as a blacklist (all matching instances get
   filtered out).
 -only-installations=false
   If set, only consider installations.
 -only-instances=false
   If set, only consider instances.
 -parallelism=BYKIND
   Specifies the level of parallelism for the handler execution. One of: [BYKIND
   FULL NONE].

Device status

Get the status of the device manager or application instances and installations.

Usage:
   device status [flags] <name patterns...>

<name patterns...> are vanadium object names or glob name patterns corresponding
to the device manager service, or to application installations and instances.

The device status flags are:
 -installation-state=
   If non-empty, specifies allowed installation states (all others installations
   get filtered out). The value of the flag is a comma-separated list of values
   from among: [Active Uninstalled]. If the value is prefixed by '!', the list
   acts as a blacklist (all matching installations get filtered out).
 -instance-state=
   If non-empty, specifies allowed instance states (all other instances get
   filtered out). The value of the flag is a comma-separated list of values from
   among: [Launching Running Dying NotRunning Updating Deleted]. If the value is
   prefixed by '!', the list acts as a blacklist (all matching instances get
   filtered out).
 -only-installations=false
   If set, only consider installations.
 -only-instances=false
   If set, only consider instances.
 -parallelism=FULL
   Specifies the level of parallelism for the handler execution. One of: [BYKIND
   FULL NONE].

Device debug

Get internal debug information about application installations and instances.

Usage:
   device debug [flags] <app name patterns...>

<app name patterns...> are vanadium object names or glob name patterns
corresponding to application installations and instances.

The device debug flags are:
 -installation-state=
   If non-empty, specifies allowed installation states (all others installations
   get filtered out). The value of the flag is a comma-separated list of values
   from among: [Active Uninstalled]. If the value is prefixed by '!', the list
   acts as a blacklist (all matching installations get filtered out).
 -instance-state=
   If non-empty, specifies allowed instance states (all other instances get
   filtered out). The value of the flag is a comma-separated list of values from
   among: [Launching Running Dying NotRunning Updating Deleted]. If the value is
   prefixed by '!', the list acts as a blacklist (all matching instances get
   filtered out).
 -only-installations=false
   If set, only consider installations.
 -only-instances=false
   If set, only consider instances.
 -parallelism=FULL
   Specifies the level of parallelism for the handler execution. One of: [BYKIND
   FULL NONE].

Device acl - Tool for setting device manager Permissions

The acl tool manages Permissions on the device manger, installations and
instances.

Usage:
   device acl [flags] <command>

The device acl commands are:
   get         Get Permissions for the given target.
   set         Set Permissions for the given target.

Device acl get

Get Permissions for the given target.

Usage:
   device acl get [flags] <device manager name>

<device manager name> can be a Vanadium name for a device manager, application
installation or instance.

Device acl set

Set Permissions for the given target

Usage:
   device acl set [flags] <device manager name>  (<blessing> [!]<tag>(,[!]<tag>)*

<device manager name> can be a Vanadium name for a device manager, application
installation or instance.

<blessing> is a blessing pattern. If the same pattern is repeated multiple times
in the command, then the only the last occurrence will be honored.

<tag> is a subset of defined access types ("Admin", "Read", "Write" etc.). If
the access right is prefixed with a '!' then <blessing> is added to the NotIn
list for that right. Using "^" as a "tag" causes all occurrences of <blessing>
in the current AccessList to be cleared.

Examples: set root/self ^ will remove "root/self" from the In and NotIn lists
for all access rights.

set root/self Read,!Write will add "root/self" to the In list for Read access
and the NotIn list for Write access (and remove "root/self" from both the In and
NotIn lists of all other access rights)

The device acl set flags are:
 -f=false
   Instead of making the AccessLists additive, do a complete replacement based
   on the specified settings.

Device publish

Publishes the given application(s) to the binary and application servers. The
binaries should be in $JIRI_ROOT/release/go/bin/[<GOOS>_<GOARCH>] by default
(can be overrriden with --from). By default the binary name is used as the name
of the application envelope, and as the title in the envelope. However,
<envelope-name> and <title> can be specified explicitly using :<envelope-name>
and @<title>. The binary is published as <binserv>/<binary
name>/<GOOS>-<GOARCH>/<TIMESTAMP>. The application envelope is published as
<appserv>/<envelope-name>/<TIMESTAMP>. Optionally, adds blessing patterns to the
Read and Resolve AccessLists.

Usage:
   device publish [flags] <binary name>[:<envelope-name>][@<title>] ...

The device publish flags are:
 -add-publisher=true
   If true, add a publisher blessing to the application envelope
 -appserv=applications
   Name of application service.
 -binserv=binaries
   Name of binary service.
 -from=
   Location of binaries to be published.  Defaults to
   $JIRI_ROOT/release/go/bin/[<GOOS>_<GOARCH>]
 -goarch=<runtime.GOARCH>
   GOARCH for application.  The default is the value of runtime.GOARCH.
 -goos=<runtime.GOOS>
   GOOS for application.  The default is the value of runtime.GOOS.
 -publisher-min-validity=30h0m0s
   Publisher blessings that are valid for less than this amount of time are
   considered invalid
 -readers=dev.v.io
   If non-empty, comma-separated blessing patterns to add to Read and Resolve
   AccessList.

Device ls

List application installations or instances.

Usage:
   device ls [flags] <app name patterns...>

<app name patterns...> are vanadium object names or glob name patterns
corresponding to application installations and instances.

The device ls flags are:
 -installation-state=
   If non-empty, specifies allowed installation states (all others installations
   get filtered out). The value of the flag is a comma-separated list of values
   from among: [Active Uninstalled]. If the value is prefixed by '!', the list
   acts as a blacklist (all matching installations get filtered out).
 -instance-state=
   If non-empty, specifies allowed instance states (all other instances get
   filtered out). The value of the flag is a comma-separated list of values from
   among: [Launching Running Dying NotRunning Updating Deleted]. If the value is
   prefixed by '!', the list acts as a blacklist (all matching instances get
   filtered out).
 -only-installations=false
   If set, only consider installations.
 -only-instances=false
   If set, only consider instances.
 -parallelism=FULL
   Specifies the level of parallelism for the handler execution. One of: [BYKIND
   FULL NONE].

Device help - Display help for commands or topics

Help with no args displays the usage of the parent command.

Help with args displays the usage of the specified sub-command or help topic.

"help ..." recursively displays help for all commands and topics.

Usage:
   device help [flags] [command/topic ...]

[command/topic ...] optionally identifies a specific sub-command or help topic.

The device help flags are:
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

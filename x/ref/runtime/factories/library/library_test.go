package library_test

import (
	"reflect"
	"testing"

	"v.io/x/ref/lib/flags"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security/access"
	"v.io/x/ref/runtime/factories/library"
)

func init() {
	library.AllowMultipleInitializations = true
}

func getValues(ctx *context.T) (rpc.ListenSpec, access.PermissionsSpec, []string) {
	return v23.GetListenSpec(ctx),
		v23.GetPermissionsSpec(ctx),
		v23.GetNamespace(ctx).Roots()
}

func TestStatic(t *testing.T) {
	ctx, shutdown := v23.Init()
	defer shutdown()
	ls, ps, roots := getValues(ctx)
	if got, want := ls.String(), `("wsh", ":0")`; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := ps.Literal, ""; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	empty := map[string]string{}
	if got, want := ps.Files, empty; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := roots, flags.DefaultNamespaceRootsNoEnv(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDefaults(t *testing.T) {
	old_protocol := flags.DefaultProtocol()
	old_hostport := flags.DefaultHostPort()
	old_proxy := flags.DefaultProxy()
	old_roots := flags.DefaultNamespaceRootsNoEnv()
	old_perms := flags.DefaultPermissions()
	old_literal := flags.DefaultPermissionsLiteral()
	defer func() {
		flags.SetDefaultProtocol(old_protocol)
		flags.SetDefaultHostPort(old_hostport)
		flags.SetDefaultProxy(old_proxy)
		flags.SetDefaultNamespaceRoots(old_roots...)
		flags.SetDefaultPermissionsLiteral(old_literal)
		for k, v := range old_perms {
			flags.SetDefaultPermissions(k, v)
		}
	}()
	flags.SetDefaultProtocol("tcp6")
	flags.SetDefaultHostPort("127.0.0.2:9999")
	flags.SetDefaultNamespaceRoots("/myroot")
	flags.SetDefaultProxy("myrpoxy")
	flags.SetDefaultPermissions("a", "b")
	flags.SetDefaultPermissions("c", "d")
	flags.SetDefaultPermissionsLiteral("{ohmy}")

	ctx, shutdown := v23.Init()
	defer shutdown()

	ls, ps, roots := getValues(ctx)

	if got, want := ls.String(), `("tcp6", "127.0.0.2:9999") proxy(myrpoxy)`; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := ps.Literal, "{ohmy}"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	perms := map[string]string{"a": "b", "c": "d"}
	if got, want := ps.Files, perms; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := roots, []string{"/myroot"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

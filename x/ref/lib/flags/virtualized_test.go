package flags_test

import (
	"flag"
	"fmt"
	"reflect"
	"testing"

	"v.io/x/ref/lib/flags"
)

func TestVirtualizedFlags(t *testing.T) {
	fl := flags.CreateAndRegister(flag.NewFlagSet("test", flag.ContinueOnError), flags.Virtualized)
	if err := fl.Parse([]string{
		"v23.virtualized.docker=true",
		"v23.virtualized.provider=foobar",
		"v23.virtualized.discover-public-address=false",
		"v23.virtualized.tcp.public-protocol=tcp",
		"v23.virtualized.tcp.public-address=8.8.2.2",
		"v23.virtualized.literal-dns-name=my-load-balancer",
	}, nil); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	fmt.Printf("WTF\n")
	expected := flags.VirtualizedFlags{}
	fmt.Printf("WTF - %#v\n", expected)
	fmt.Printf("WTF - %#v\n", fl)
	x := fl.VirtualizedFlags()
	fmt.Printf("WTF - %#v\n", x.DiscoverPublicIP)
	fmt.Printf("WTF - %#v\n", x.Dockerized)
	fmt.Printf("WTF - %#v\n", x.VirtualizationProvider)

	fmt.Printf("WTF - %#v\n", x.LiteralDNSName)
	fmt.Printf("WTF - %#v\n", x.PublicAddress)
	fmt.Printf("WTF - %#v\n", x.PublicProtocol)
	fmt.Printf("WTF  DONE\n")

	if got, want := fl.VirtualizedFlags(), expected; !reflect.DeepEqual(got, want) {
		fmt.Printf("WTF - %#v\n", got)
		fmt.Printf("WTF - %#v\n", want)

		//t.Errorf("got %v, want %v", got, want)
	}

	t.Fail()
}

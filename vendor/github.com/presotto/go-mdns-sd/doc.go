package mdns

/*
Package MDNS provides an MDNS interface (RFC 6762) that is both a client and a server.  It caches all
resource records learned from any network it is listening to.  Depending on the loopback flag in NewMDNS,
it will either listen to all non-loopback or to only loopback network interfaces and, in both cases,
only on ones that have ipv4 or ipv6 addresses.

An instance is created by:

	s, err := NewMDNS(hostname,
			  ipv4 address - default "224.0.0.251:5353",
			  ipv6 address - default "[FF02::FB]:5353",
			  true if using only loopback (i.e. testing)
			  true if we want extensive logging)

To register interest in a service (i.e. for service discovery ala RFC 6763):

	s.SubscribeToService(service name)

This actually sends a multicast request on all networks asking for anyone providing that service.
It need only be done once, since, as systems join the network they will multicast that information.

To register as a provider of a service:

	s.AddService(servicename,
		     hostname - default, the host name provided with NewMDNS,
		     port)

To learn all providers of a service:

	var instances []mdns.ServiceInstance
	instances = s.ServiceDiscovery(service name)

where

	type ServiceInstance struct {
		Target string
		Port   uint16
	}

To learn the addresses of a host:

	var ips []net.IP
	ips = s.ResolveAddress(domain name - can be with or without a trailing ".local")

To learn an RR (dns resource record) of a particular type:

	var rrs []dns.RR
	rrs = s.ResolveRR(domain name - can be with or without a trailing ".local")

To stop the service:

	s.Stop()


*/

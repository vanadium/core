// +build windows

package dns

import (
	"errors"
	"net"
)

func LookupHost(name string) (addrs []string, err error) {
	return nil, errors.New("not implemented")
}

func LookupSRV(service, proto, name string) (cname string, addrs []*net.SRV, err error) {
	return "", nil, errors.New("not implemented")
}

func LookupMX(name string) (mx []*net.MX, err error) {
	return nil, errors.New("not implemented")
}

func LookupTXT(name string) (txt []string, err error) {
	return nil, errors.New("not implemented")
}

func LookupAddr(addr string) (name []string, err error) {
	return nil, errors.New("not implemented")
}
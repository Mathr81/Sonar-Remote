// Package mdnsx advertises the server over multicast DNS so devices on the
// LAN (iPad/iPhone resolve .local natively) can reach it at <host>.local.
package mdnsx

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/grandcat/zeroconf"
)

// Advertise publishes <host>.local (A record) + an _http._tcp service on the
// given port, and returns a shutdown func plus a short label of the method used.
//
// It advertises ONLY the primary LAN IP on ONLY its interface. Publishing every
// adapter (VPN/WSL/VirtualBox/link-local) makes clients pick an unreachable
// address and hang, so we deliberately restrict it.
//
// If Apple Bonjour (dns-sd) is present we register THROUGH it: running our own
// responder alongside Bonjour on port 5353 produces conflicting answers that
// never reach the OS resolver (browsers/iOS). Bonjour is what iOS trusts, so we
// use it when available and fall back to the built-in responder otherwise.
func Advertise(host string, port int) (func(), string, error) {
	ip, iface := primaryInterface()
	if ip == nil {
		return nil, "", fmt.Errorf("no primary LAN IPv4 address found")
	}

	if stop, err := viaBonjour(host, port, ip.String()); err == nil {
		return stop, "Bonjour (dns-sd)", nil
	} else if !errors.Is(err, errNoBonjour) {
		log.Printf("mDNS: dns-sd failed (%v); using built-in responder", err)
	}

	var ifaces []net.Interface
	if iface != nil {
		ifaces = []net.Interface{*iface}
	}
	server, err := zeroconf.RegisterProxy(
		"Sonar Remote", "_http._tcp", "local.", port,
		host, []string{ip.String()}, []string{"path=/"}, ifaces,
	)
	if err != nil {
		return nil, "", err
	}
	return server.Shutdown, "built-in responder", nil
}

var errNoBonjour = errors.New("dns-sd not found")

// viaBonjour registers the host via Apple's dns-sd proxy registration, held
// alive by the child process for as long as we run.
func viaBonjour(host string, port int, ip string) (func(), error) {
	path, err := exec.LookPath("dns-sd")
	if err != nil {
		return nil, errNoBonjour
	}
	cmd := exec.Command(path, "-P",
		"Sonar Remote", "_http._tcp", "local",
		strconv.Itoa(port), host+".local.", ip)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	tieToOurLifetime(cmd.Process)
	// If it dies immediately, the registration is bad -> report so we fall back.
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	select {
	case err := <-exited:
		return nil, fmt.Errorf("dns-sd exited early: %v", err)
	case <-time.After(600 * time.Millisecond):
	}
	return func() { _ = cmd.Process.Kill() }, nil
}

// primaryInterface returns the IPv4 address (and its interface) that the OS
// would use to reach the network, i.e. the one on the default route.
func primaryInterface() (net.IP, *net.Interface) {
	conn, err := net.Dial("udp", "8.8.8.8:80") // no packets sent; picks source IP
	if err != nil {
		return firstPrivateIPv4()
	}
	defer conn.Close()
	ip := conn.LocalAddr().(*net.UDPAddr).IP.To4()
	if ip == nil {
		return firstPrivateIPv4()
	}
	ifaces, _ := net.Interfaces()
	for i := range ifaces {
		addrs, _ := ifaces[i].Addrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil && n.IP.Equal(ip) {
				return ip, &ifaces[i]
			}
		}
	}
	return ip, nil
}

func firstPrivateIPv4() (net.IP, *net.Interface) {
	ifaces, _ := net.Interfaces()
	for i := range ifaces {
		if ifaces[i].Flags&net.FlagUp == 0 || ifaces[i].Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifaces[i].Addrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok {
				if ip4 := n.IP.To4(); ip4 != nil && ip4.IsPrivate() {
					return ip4, &ifaces[i]
				}
			}
		}
	}
	return nil, nil
}

// LocalIPv4 returns this host's non-loopback IPv4 addresses.
func LocalIPv4() []string {
	var out []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, ifi := range ifaces {
		if ifi.Flags&net.FlagUp == 0 || ifi.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifi.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip4 := ip.To4(); ip4 != nil && !ip4.IsLoopback() {
				out = append(out, ip4.String())
			}
		}
	}
	return out
}

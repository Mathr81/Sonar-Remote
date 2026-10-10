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
// given port for as long as the returned stop func isn't called. It follows the
// network: it waits for a LAN address at boot and re-registers when it changes.
//
// It advertises ONLY the primary LAN IP on ONLY its interface. Publishing every
// adapter (VPN/WSL/VirtualBox/link-local) makes clients pick an unreachable
// address and hang, so we deliberately restrict it.
func Advertise(host string, port int) (stop func()) {
	done := make(chan struct{})
	go func() {
		var current string
		var stopCurrent func()
		tick := time.NewTicker(10 * time.Second)
		defer tick.Stop()
		for {
			ip, iface := primaryInterface()
			if addr := ip.String(); ip != nil && addr != current {
				if stopCurrent != nil {
					stopCurrent()
				}
				current, stopCurrent = "", nil
				if s, method, err := register(host, port, addr, iface); err != nil {
					log.Printf("mDNS: %v", err)
				} else {
					current, stopCurrent = addr, s
					log.Printf("mDNS: %s.local -> %s via %s", host, addr, method)
				}
			}
			select {
			case <-done:
				if stopCurrent != nil {
					stopCurrent()
				}
				return
			case <-tick.C:
			}
		}
	}()
	return func() { close(done) }
}

// register publishes one address. If Apple Bonjour (dns-sd) is present we
// register THROUGH it: running our own responder alongside Bonjour on port 5353
// produces conflicting answers that never reach the OS resolver (browsers/iOS).
// Bonjour is what iOS trusts, so we use it when available and fall back to the
// built-in responder otherwise.
func register(host string, port int, ip string, iface *net.Interface) (func(), string, error) {
	if stop, err := viaBonjour(host, port, ip); err == nil {
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
		host, []string{ip}, []string{"path=/"}, ifaces,
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

// primaryInterface returns the IPv4 address (and its interface) on the default
// route, or nil while the machine has no network.
func primaryInterface() (net.IP, *net.Interface) {
	conn, err := net.Dial("udp", "8.8.8.8:80") // no packets sent; picks source IP
	if err != nil {
		return nil, nil
	}
	defer conn.Close()
	ip := conn.LocalAddr().(*net.UDPAddr).IP.To4()
	if ip == nil {
		return nil, nil
	}
	ifaces, _ := net.Interfaces()
	for i := range ifaces {
		addrs, _ := ifaces[i].Addrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.IP.Equal(ip) {
				return ip, &ifaces[i]
			}
		}
	}
	return ip, nil
}

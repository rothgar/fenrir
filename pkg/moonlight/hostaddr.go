package moonlight

import (
	"net"
	"net/http"
)

const zeroMAC = "00:00:00:00:00:00"

// interfaceLister is net.Interfaces, swappable in tests.
type interfaceLister func() ([]net.Interface, error)

// hostIdentity returns the MAC and IP that Moonlight should store for this
// host. Moonlight uses the MAC for "Wake PC" (Wake-on-LAN) and LocalIP for
// LAN discovery, so they must be the host NIC the client actually reached.
//
// The proxy runs with hostNetwork, so the request's local address is the
// host's own IP; we find the interface owning it and report its MAC.
// macOverride (from --host-mac / HOST_MAC) wins when set, e.g. when the
// request arrives on a bridge or VLAN interface whose MAC isn't the one
// the NIC firmware listens for.
func hostIdentity(r *http.Request, macOverride string, ifaces interfaceLister) (mac, ip string) {
	mac, ip = zeroMAC, "127.0.0.1"
	if a, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr); ok {
		if host, _, err := net.SplitHostPort(a.String()); err == nil {
			if parsed := net.ParseIP(host); parsed != nil {
				if v4 := parsed.To4(); v4 != nil {
					parsed = v4
				}
				ip = parsed.String()
				if m := macForIP(parsed, ifaces); m != "" {
					mac = m
				}
			}
		}
	}
	if macOverride != "" {
		if hw, err := net.ParseMAC(macOverride); err == nil {
			mac = hw.String()
		}
	}
	return mac, ip
}

func macForIP(ip net.IP, ifaces interfaceLister) string {
	list, err := ifaces()
	if err != nil {
		return ""
	}
	for _, iface := range list {
		if len(iface.HardwareAddr) != 6 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.IP.Equal(ip) {
				return iface.HardwareAddr.String()
			}
		}
	}
	return ""
}

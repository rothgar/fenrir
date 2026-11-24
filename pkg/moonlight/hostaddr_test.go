package moonlight

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// withLocalAddr returns a request as if it arrived on local address addr.
func withLocalAddr(addr string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/serverinfo", nil)
	tcp, _ := net.ResolveTCPAddr("tcp", addr)
	return r.WithContext(context.WithValue(r.Context(), http.LocalAddrContextKey, tcp))
}

func TestMacForIPSkipsInterfacesWithoutMAC(t *testing.T) {
	// Loopback has no 6-byte hardware address, so it must never be reported.
	if m := macForIP(net.ParseIP("127.0.0.1"), net.Interfaces); m != "" {
		t.Fatalf("loopback has no MAC; got %q", m)
	}
}

func TestHostIdentityReportsLocalIPAndOverride(t *testing.T) {
	noIfaces := func() ([]net.Interface, error) { return nil, nil }

	mac, ip := hostIdentity(withLocalAddr("10.1.3.20:47989"), "", noIfaces)
	if ip != "10.1.3.20" || mac != zeroMAC {
		t.Fatalf("got mac=%s ip=%s", mac, ip)
	}

	mac, _ = hostIdentity(withLocalAddr("10.1.3.20:47989"), "A8-A1-59-12-34-56", noIfaces)
	if mac != "a8:a1:59:12:34:56" {
		t.Fatalf("override not applied/normalised: %s", mac)
	}

	mac, _ = hostIdentity(withLocalAddr("10.1.3.20:47989"), "not-a-mac", noIfaces)
	if mac != zeroMAC {
		t.Fatalf("invalid override should be ignored: %s", mac)
	}

	// IPv4-mapped IPv6 local addresses (dual-stack listeners) are reported as IPv4.
	_, ip = hostIdentity(withLocalAddr("[::ffff:10.1.3.20]:47989"), "", noIfaces)
	if ip != "10.1.3.20" {
		t.Fatalf("ipv4-mapped address: %s", ip)
	}

	// No local address in context (shouldn't happen with net/http): safe defaults.
	mac, ip = hostIdentity(httptest.NewRequest(http.MethodGet, "/", nil), "", noIfaces)
	if mac != zeroMAC || ip != "127.0.0.1" {
		t.Fatalf("defaults: %s %s", mac, ip)
	}
}

// TestHostIdentityDetectsRealInterface binds a listener on a real non-loopback
// interface (if the test machine has one) and checks the reported MAC is
// that interface's.
func TestHostIdentityDetectsRealInterface(t *testing.T) {
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if len(iface.HardwareAddr) != 6 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			n, ok := a.(*net.IPNet)
			if !ok || n.IP.To4() == nil || n.IP.IsLoopback() {
				continue
			}
			mac, ip := hostIdentity(withLocalAddr(net.JoinHostPort(n.IP.String(), "47989")), "", net.Interfaces)
			if ip != n.IP.String() || mac != iface.HardwareAddr.String() {
				t.Fatalf("iface %s: got mac=%s ip=%s want %s %s", iface.Name, mac, ip, iface.HardwareAddr, n.IP)
			}
			return
		}
	}
	t.Skip("no non-loopback IPv4 interface with a MAC on this machine")
}

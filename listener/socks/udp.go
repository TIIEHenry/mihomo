package socks

import (
	"context"
	"net"
	"sync"

	"github.com/metacubex/mihomo/adapter/inbound"
	N "github.com/metacubex/mihomo/common/net"
	"github.com/metacubex/mihomo/common/sockopt"
	"github.com/metacubex/mihomo/component/auth"
	C "github.com/metacubex/mihomo/constant"
	authStore "github.com/metacubex/mihomo/listener/auth"
	LC "github.com/metacubex/mihomo/listener/config"
	"github.com/metacubex/mihomo/log"
	"github.com/metacubex/mihomo/transport/socks5"
)

type UDPListener struct {
	packetConn net.PacketConn
	addr       string
	closed     bool
}

// RawAddress implements C.Listener
func (l *UDPListener) RawAddress() string {
	return l.addr
}

// Address implements C.Listener
func (l *UDPListener) Address() string {
	return l.packetConn.LocalAddr().String()
}

// Close implements C.Listener
func (l *UDPListener) Close() error {
	l.closed = true
	return l.packetConn.Close()
}

func NewUDP(addr string, tunnel C.Tunnel, assoc *UDPAssoc, additions ...inbound.Addition) (*UDPListener, error) {
	return NewUDPWithConfig(defaultConfig(addr), inbound.NewListenConfig(), tunnel, assoc, additions...)
}

func NewUDPWithConfig(config LC.AuthServer, lc C.InboundListenConfig, tunnel C.Tunnel, assoc *UDPAssoc, additions ...inbound.Addition) (*UDPListener, error) {
	isDefault := false
	if len(additions) == 0 {
		isDefault = true
		additions = []inbound.Addition{
			inbound.WithInName("DEFAULT-SOCKS"),
			inbound.WithSpecialRules(""),
		}
	}
	l, err := lc.ListenPacket(context.Background(), "udp", config.Listen)
	if err != nil {
		return nil, err
	}

	if err := sockopt.UDPReuseaddr(l); err != nil {
		log.Warnln("Failed to Reuse UDP Address: %s", err)
	}

	sl := &UDPListener{
		packetConn: l,
		addr:       config.Listen,
	}
	conn := N.NewEnhancePacketConn(l)
	store := config.AuthStore
	go func() {
		for {
			data, put, remoteAddr, err := conn.WaitReadFrom()
			if err != nil {
				if put != nil {
					put()
				}
				if sl.closed {
					break
				}
				continue
			}
			handleSocksUDP(l, tunnel, data, put, remoteAddr, assoc, store, isDefault, additions...)
		}
	}()

	return sl, nil
}

func handleSocksUDP(pc net.PacketConn, tunnel C.Tunnel, buf []byte, put func(), addr net.Addr, assoc *UDPAssoc, store auth.AuthStore, isDefault bool, additions ...inbound.Addition) {
	target, payload, err := socks5.DecodeUDPPacket(buf)
	if err != nil {
		// Unresolved UDP packet, return buffer to the pool
		if put != nil {
			put()
		}
		return
	}
	if store != nil && store.Authenticator() != nil && !((isDefault || store == authStore.Default) && inbound.SkipAuthRemoteAddr(addr)) {
		ip := ipKey(addrIP(addr))
		if ip == "" || assoc == nil || !assoc.has(ip) {
			if put != nil {
				put()
			}
			return
		}
	}
	packet := &packet{
		pc:      pc,
		rAddr:   addr,
		payload: payload,
		put:     put,
	}
	tunnel.HandleUDPPacket(inbound.NewPacket(target, packet, C.SOCKS5, additions...))
}

type UDPAssoc struct {
	mu sync.Mutex
	n  map[string]int
}

func NewUDPAssoc() *UDPAssoc {
	return &UDPAssoc{n: make(map[string]int)}
}

func (a *UDPAssoc) add(ip string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.n[ip]++
}

func (a *UDPAssoc) del(ip string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.n[ip] <= 1 {
		delete(a.n, ip)
		return
	}
	a.n[ip]--
}

func (a *UDPAssoc) has(ip string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.n[ip] > 0
}

// udpAssocKeys lists the source addresses an authenticated UDP ASSOCIATE may
// send from. The TCP peer is always allowed. A concrete address in the request
// is allowed as well when it differs from that peer. A domain, or an
// unspecified address, does not name a source, so only the peer is used.
func udpAssocKeys(target socks5.Addr, conn net.Conn) []string {
	peer := ""
	if conn != nil && conn.RemoteAddr() != nil {
		peer = ipKey(addrIP(conn.RemoteAddr()))
	}
	declared := ""
	if len(target) == 0 || target[0] != socks5.AtypDomainName {
		if udpAddr := target.UDPAddr(); udpAddr != nil && len(udpAddr.IP) > 0 && !udpAddr.IP.IsUnspecified() {
			declared = ipKey(udpAddr.IP)
		}
	}
	keys := make([]string, 0, 2)
	if peer != "" {
		keys = append(keys, peer)
	}
	if declared != "" && declared != peer {
		keys = append(keys, declared)
	}
	return keys
}

func trackUDPAssoc(target socks5.Addr, conn net.Conn, assoc *UDPAssoc) func() {
	keys := udpAssocKeys(target, conn)
	for _, ip := range keys {
		assoc.add(ip)
	}
	return func() {
		for _, ip := range keys {
			assoc.del(ip)
		}
	}
}

func addrIP(addr net.Addr) net.IP {
	switch a := addr.(type) {
	case *net.TCPAddr:
		return a.IP
	case *net.UDPAddr:
		return a.IP
	default:
		host, _, err := net.SplitHostPort(addr.String())
		if err != nil {
			return net.ParseIP(addr.String())
		}
		return net.ParseIP(host)
	}
}

func ipKey(ip net.IP) string {
	if ip == nil {
		return ""
	}
	if ip4 := ip.To4(); ip4 != nil {
		return ip4.String()
	}
	return ip.String()
}

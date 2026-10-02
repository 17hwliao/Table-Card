package server

import (
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

type ConnectionAddress struct {
	Address   string `json:"address"`
	Interface string `json:"interface"`
}
type ConnectionInfo struct {
	Local     string              `json:"local"`
	Addresses []ConnectionAddress `json:"addresses"`
}

// ConnectionInfoFor uses the actual listening address. A loopback-only server
// must never advertise LAN addresses on which it does not accept connections.
func ConnectionInfoFor(listener net.Addr) ConnectionInfo {
	info := ConnectionInfo{Addresses: []ConnectionAddress{}}
	tcp, ok := listener.(*net.TCPAddr)
	if !ok {
		return info
	}
	port := strconv.Itoa(tcp.Port)
	bound := tcp.IP
	if len(bound) == 0 || bound.IsUnspecified() {
		info.Local = net.JoinHostPort("127.0.0.1", port)
	} else if bound.IsLoopback() {
		info.Local = net.JoinHostPort(bound.String(), port)
		return info
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return info
	}
	seen := map[string]bool{}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err != nil || ip.To4() == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
				continue
			}
			if len(bound) > 0 && !bound.IsUnspecified() && !bound.Equal(ip) {
				continue
			}
			endpoint := net.JoinHostPort(ip.String(), port)
			if seen[endpoint] {
				continue
			}
			seen[endpoint] = true
			info.Addresses = append(info.Addresses, ConnectionAddress{Address: endpoint, Interface: iface.Name})
		}
	}
	// Named IPv6-only bindings are displayed too. Wildcard LAN discovery keeps
	// IPv4 addresses, which are easier to copy into the Windows client launcher.
	if len(bound) > 0 && !bound.IsUnspecified() && len(info.Addresses) == 0 {
		info.Addresses = append(info.Addresses, ConnectionAddress{Address: net.JoinHostPort(bound.String(), port), Interface: "指定监听网卡"})
	}
	sort.Slice(info.Addresses, func(i, j int) bool {
		left, right := connectionPriority(info.Addresses[i].Interface), connectionPriority(info.Addresses[j].Interface)
		if left != right {
			return left < right
		}
		return info.Addresses[i].Address < info.Addresses[j].Address
	})
	return info
}
func connectionPriority(name string) int {
	name = strings.ToLower(name)
	for _, virtual := range []string{"virtual", "vethernet", "vmware", "vpn", "wsl", "docker", "loopback", "tailscale", "zerotier"} {
		if strings.Contains(name, virtual) {
			return 2
		}
	}
	for _, physical := range []string{"wi-fi", "wifi", "wlan", "ethernet", "以太网", "无线"} {
		if strings.Contains(name, physical) {
			return 0
		}
	}
	return 1
}

func (s *Server) SetListenAddress(address net.Addr) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listenAddress = address
}
func (s *Server) connectionInfo(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	address := s.listenAddress
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, ConnectionInfoFor(address))
}

//go:build windows
// +build windows

package p2p

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	routedhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/soitun/mynetwork/config"
)

// -----------------------------------------------------------------------------------------------------------------------------------------------------
type ParallelRouting struct {
	routings []routedhost.Routing
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func (pr ParallelRouting) FindPeer(ctx context.Context, p peer.ID) (peer.AddrInfo, error) {
	var wg sync.WaitGroup
	var mutex sync.Mutex

	var info peer.AddrInfo
	info.ID = p
	subCtx, cancelSubCtx := context.WithTimeout(ctx, 30*time.Second)
	for _, r := range pr.routings {
		wg.Add(1)
		go func(r routedhost.Routing) {
			defer wg.Done()
			if pinfo, err := r.FindPeer(subCtx, p); err == nil {
				mutex.Lock()
				info.Addrs = append(info.Addrs, pinfo.Addrs...)
				mutex.Unlock()
			}
		}(r)
	}
	wg.Wait()
	cancelSubCtx()

	if len(info.Addrs) == 0 {
		return info, routing.ErrNotFound
	}

	return info, nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
type RecursionGater struct {
	config  *config.Config
	ifindex int
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func NewRecursionGater(config *config.Config) RecursionGater {
	// On Windows, we don't use netlink, so we set ifindex to 0
	return RecursionGater{
		config:  config,
		ifindex: 0,
	}
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func (rg RecursionGater) InterceptAddrDial(pid peer.ID, addr ma.Multiaddr) bool {
	if ip4str, err := addr.ValueForProtocol(ma.P_IP4); err == nil {
		ip4 := net.ParseIP(ip4str)
		if rte, ok := rg.config.FindRouteForIP(ip4); ok {
			if rte.Target.ID == pid {
				return false
			}
		}
	}
	return true
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func (rg RecursionGater) InterceptPeerDial(pid peer.ID) bool {
	return true
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func (rg RecursionGater) InterceptAccept(addrs network.ConnMultiaddrs) bool {
	return true
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func (rg RecursionGater) InterceptSecured(direction network.Direction, pid peer.ID, addrs network.ConnMultiaddrs) bool {
	return true
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func (rg RecursionGater) InterceptUpgraded(network.Conn) (bool, control.DisconnectReason) {
	return true, 0
}

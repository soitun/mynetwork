//go:build !windows
// +build !windows

package rpc

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"syscall"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/soitun/mynetwork/config"
	"github.com/soitun/mynetwork/p2p"
	"github.com/soitun/mynetwork/tun"
	"github.com/yl2chen/cidranger"
)

// -----------------------------------------------------------------------------------------------------------------------------------------------------
type HyprspaceRPC struct {
	host   host.Host
	config *config.Config
	tunDev *tun.TUN
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func (hsr *HyprspaceRPC) Status(args *Args, reply *StatusReply) error {
	netPeersCurrent := 0
	var netPeerAddrsCurrent []string
	for _, p := range hsr.config.Peers {
		if hsr.host.Network().Connectedness(p.ID) == network.Connected {
			netPeersCurrent = netPeersCurrent + 1
			for _, c := range hsr.host.Network().ConnsToPeer(p.ID) {
				netPeerAddrsCurrent = append(netPeerAddrsCurrent, fmt.Sprintf("@%s (%s) %s/p2p/%s",
					p.Name,
					hsr.host.Peerstore().LatencyEWMA(p.ID).String(),
					c.RemoteMultiaddr().String(),
					p.ID.String(),
				))
			}
		}
	}
	var addrStrings []string
	for _, ma := range hsr.host.Addrs() {
		addrStrings = append(addrStrings, ma.String())
	}
	*reply = StatusReply{
		hsr.host.ID().String(),
		len(hsr.host.Network().Conns()),
		netPeersCurrent,
		netPeerAddrsCurrent,
		len(hsr.config.Peers),
		addrStrings,
	}
	return nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func (hsr *HyprspaceRPC) Route(args *RouteArgs, reply *RouteReply) error {
	switch args.Action {
	case Show:
		var routeInfos []RouteInfo
		allRoutes4, err := hsr.config.PeerLookup.ByRoute.CoveredNetworks(*cidranger.AllIPv4)
		if err != nil {
			return err
		}
		allRoutes6, err := hsr.config.PeerLookup.ByRoute.CoveredNetworks(*cidranger.AllIPv6)
		if err != nil {
			return err
		}
		allRoutes := append(allRoutes4, allRoutes6...)
		for _, r := range allRoutes {
			rte := *r.(*config.RouteTableEntry)
			connected := hsr.host.Network().Connectedness(rte.Target.ID) == network.Connected
			relay := false
			relayAddr := rte.Target.ID
			if connected {
			ConnLoop:
				for _, c := range hsr.host.Network().ConnsToPeer(rte.Target.ID) {
					for _, s := range c.GetStreams() {
						if s.Protocol() == p2p.Protocol {
							if _, err := c.RemoteMultiaddr().ValueForProtocol(multiaddr.P_CIRCUIT); err == nil {
								relay = true
								if ra, err := c.RemoteMultiaddr().ValueForProtocol(multiaddr.P_P2P); err == nil {
									relayAddr, err = peer.Decode(ra)
									if err != nil {
										relayAddr = rte.Target.ID
									}
								}
							} else {
								relay = false
								relayAddr = rte.Target.ID
								break ConnLoop
							}
						}
					}
				}
			}
			routeInfos = append(routeInfos, RouteInfo{
				Network:     rte.Network(),
				TargetName:  rte.Target.Name,
				TargetAddr:  rte.Target.ID,
				RelayAddr:   relayAddr,
				IsRelay:     relay,
				IsConnected: connected,
			})
		}
		*reply = RouteReply{
			Routes: routeInfos,
		}
	case Add:
		if len(args.Args) != 2 {
			return errors.New("expected exactly 2 arguments")
		}
		_, network, err := net.ParseCIDR(args.Args[0])
		if err != nil {
			return err
		}
		target, found := config.FindPeerByCLIRef(hsr.config.Peers, args.Args[1])
		if !found {
			return errors.New("no such peer")
		}
		err = hsr.tunDev.Apply(tun.Route(*network))
		if err != nil {
			return err
		}

		hsr.config.PeerLookup.ByRoute.Insert(&config.RouteTableEntry{
			Net:    *network,
			Target: *target,
		})
	case Del:
		if len(args.Args) != 1 {
			return errors.New("expected exactly 1 argument")
		}
		_, network, err := net.ParseCIDR(args.Args[0])
		if err != nil {
			return err
		}

		err = hsr.tunDev.Apply(tun.RemoveRoute(*network))
		if err != nil {
			return err
		}

		_, err = hsr.config.PeerLookup.ByRoute.Remove(*network)
		if err != nil {
			_ = hsr.tunDev.Apply(tun.Route(*network))
			return err
		}
	default:
		return errors.New("no such action")
	}
	return nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func (hsr *HyprspaceRPC) Peers(args *Args, reply *PeersReply) error {
	var peers []PeerInfo
	
	for _, c := range hsr.host.Network().Conns() {
		peerID := c.RemotePeer().String()
		
		// 获取节点的 IP 地址信息
		peerInfo := PeerInfo{
			PeerID: peerID,
			Name:   "", // 先设置为空，后面从配置中获取节点名称
		}
		
		// 从配置中查找对应的 IPv4 和 IPv6 地址以及节点名称
		for _, peer := range hsr.config.Peers {
			if peer.ID.String() == peerID {
				peerInfo.Name = peer.Name // 只保留节点名称
				if peer.BuiltinAddr4 != nil {
					peerInfo.IPv4 = peer.BuiltinAddr4.String()
				}
				if peer.BuiltinAddr6 != nil {
					peerInfo.IPv6 = peer.BuiltinAddr6.String()
				}
				break
			}
		}
		
		peers = append(peers, peerInfo)
	}
	
	*reply = PeersReply{Peers: peers}
	return nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func (hsr *HyprspaceRPC) AddPeer(args *AddPeerArgs, reply *AddPeerReply) error {
	// 验证输入参数
	if args.Name == "" {
		*reply = AddPeerReply{Success: false, Message: "Peer name cannot be empty", Err: errors.New("peer name cannot be empty")}
		return nil
	}
	if args.ID == "" {
		*reply = AddPeerReply{Success: false, Message: "Peer ID cannot be empty", Err: errors.New("peer ID cannot be empty")}
		return nil
	}

	// 解析 peer ID
	peerID, err := peer.Decode(args.ID)
	if err != nil {
		*reply = AddPeerReply{Success: false, Message: fmt.Sprintf("Invalid peer ID: %v", err), Err: err}
		return nil
	}

	// 检查 peer 是否已存在
	if _, found := config.FindPeer(hsr.config.Peers, peerID); found {
		*reply = AddPeerReply{Success: false, Message: "Peer already exists", Err: errors.New("peer already exists")}
		return nil
	}

	// 检查名称是否已存在
	if _, found := config.FindPeerByName(hsr.config.Peers, args.Name); found {
		*reply = AddPeerReply{Success: false, Message: "Peer name already exists", Err: errors.New("peer name already exists")}
		return nil
	}

	// 调用配置模块的方法添加 peer
	err = hsr.config.AddPeer(args.Name, peerID)
	if err != nil {
		*reply = AddPeerReply{Success: false, Message: fmt.Sprintf("Failed to add peer: %v", err), Err: err}
		return nil
	}

	// 获取新添加的 peer 并添加路由到 TUN 设备
	newPeer := hsr.config.Peers[len(hsr.config.Peers)-1]
	
	// 添加 IPv4 路由到 TUN 设备
	ipv4Route := net.IPNet{
		IP:   newPeer.BuiltinAddr4,
		Mask: net.CIDRMask(32, 32),
	}
	err = hsr.tunDev.Apply(tun.Route(ipv4Route))
	if err != nil {
		fmt.Printf("[!] Warning: Failed to add IPv4 route to TUN device: %v\n", err)
	}

	// 添加 IPv6 路由到 TUN 设备
	ipv6Route := net.IPNet{
		IP:   newPeer.BuiltinAddr6,
		Mask: net.CIDRMask(128, 128),
	}
	err = hsr.tunDev.Apply(tun.Route(ipv6Route))
	if err != nil {
		fmt.Printf("[!] Warning: Failed to add IPv6 route to TUN device: %v\n", err)
	}

	// 触发重新发现，让新添加的节点能被发现服务识别
	p2p.Rediscover()

	*reply = AddPeerReply{Success: true, Message: fmt.Sprintf("Peer %s (%s) added successfully", args.Name, args.ID), Err: nil}
	return nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------

// NodeIp 返回当前节点的 IP 地址
func (hsr *HyprspaceRPC) NodeIp(args *Args, reply *NodeIpReply) error {
	// 获取节点的内置 IPv4 和 IPv6 地址
	if hsr.config.BuiltinAddr4 != nil {
		reply.IPv4 = hsr.config.BuiltinAddr4.String()
	}
	if hsr.config.BuiltinAddr6 != nil {
		reply.IPv6 = hsr.config.BuiltinAddr6.String()
	}
	return nil
}

func RpcServer(ctx context.Context, ma multiaddr.Multiaddr, host host.Host, config *config.Config, tunDev *tun.TUN) {
	hsr := HyprspaceRPC{host, config, tunDev}
	rpc.Register(&hsr)

	addr, err := ma.ValueForProtocol(multiaddr.P_UNIX)
	if err != nil {
		log.Fatal("[!] Failed to parse multiaddr: ", err)
	}

	var l net.Listener
	oldUmask := syscall.Umask(0o007)

	var lc net.ListenConfig
	l, err = lc.Listen(ctx, "unix", addr)
	syscall.Umask(oldUmask)

	if err != nil {
		log.Fatal("[!] Failed to launch RPC server: ", err)
	}

	fmt.Println("[-] RPC server ready")
	go rpc.Accept(l)
	<-ctx.Done()
	fmt.Println("[-] Closing RPC server")
	l.Close()
}

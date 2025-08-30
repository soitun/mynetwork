//go:build windows
// +build windows

package rpc

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"path/filepath"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/soitun/mynetwork/config"
	"github.com/soitun/mynetwork/p2p"
	"github.com/soitun/mynetwork/tun"
)

// -----------------------------------------------------------------------------------------------------------------------------------------------------
type HyprspaceRPC struct {
	host   host.Host
	config *config.Config
	tunDev *tun.TUN
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func (hsr *HyprspaceRPC) Status(args *Args, reply *StatusReply) error {
	reply.PeerID = hsr.host.ID().String()
	reply.ListenAddrs = make([]string, len(hsr.host.Addrs()))
	for i, addr := range hsr.host.Addrs() {
		reply.ListenAddrs[i] = addr.String()
	}

	connectedPeers := hsr.host.Network().Peers()
	reply.SwarmPeersCurrent = len(connectedPeers)
	reply.NetPeersCurrent = len(connectedPeers)
	reply.NetPeerAddrsCurrent = make([]string, len(connectedPeers))
	for i, p := range connectedPeers {
		reply.NetPeerAddrsCurrent[i] = p.String()
	}
	reply.NetPeersMax = 100 // Default max peers

	return nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func (hsr *HyprspaceRPC) Route(args *RouteArgs, reply *RouteReply) error {
	if len(args.Args) == 0 {
		reply.Err = errors.New("no arguments provided")
		return nil
	}

	switch args.Action {
	case Show:
		// Show current routes
		reply.Out = "Current routes:\n"
		reply.Routes = []RouteInfo{}
		// TODO: Implement route listing
	case Add:
		if len(args.Args) < 1 {
			reply.Err = errors.New("destination required for add action")
			return nil
		}
		dest := args.Args[0]

		// Parse destination
		var targetPeer peer.ID
		var err error

		// Try to decode as peer ID
		if targetPeer, err = peer.Decode(dest); err != nil {
			// Try to find by name in config
			for _, p := range hsr.config.Peers {
				if p.Name == dest {
					targetPeer = p.ID
					err = nil
					break
				}
			}
			if err != nil {
				reply.Err = fmt.Errorf("peer not found: %s", dest)
				return nil
			}
		}

		// Check if peer is connected
		connectedPeers := hsr.host.Network().Peers()
		var isConnected bool
		for _, p := range connectedPeers {
			if p == targetPeer {
				isConnected = true
				break
			}
		}

		if !isConnected {
			// Try to connect
			ctx := context.Background()
			addrInfo := peer.AddrInfo{ID: targetPeer}
			if err := hsr.host.Connect(ctx, addrInfo); err != nil {
				reply.Err = fmt.Errorf("failed to connect to peer: %v", err)
				return nil
			}
		}

		reply.Out = fmt.Sprintf("Successfully connected to peer %s", targetPeer.String())
	case Del:
		// TODO: Implement route deletion
		reply.Out = "Route deletion not implemented on Windows"
	default:
		reply.Err = fmt.Errorf("unknown action: %s", args.Action)
	}

	return nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func (hsr *HyprspaceRPC) Peers(args *Args, reply *PeersReply) error {
	reply.Peers = make([]PeerInfo, len(hsr.config.Peers))
	
	for i, p := range hsr.config.Peers {
		// 创建包含 IP 地址信息的 PeerInfo
		peerInfo := PeerInfo{
			PeerID: p.ID.String(),
			Name:   p.Name, // 只保留节点名称
		}
		
		if p.BuiltinAddr4 != nil {
			peerInfo.IPv4 = p.BuiltinAddr4.String()
		}
		if p.BuiltinAddr6 != nil {
			peerInfo.IPv6 = p.BuiltinAddr6.String()
		}
		
		reply.Peers[i] = peerInfo
	}
	return nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func (hsr *HyprspaceRPC) AddPeer(args *AddPeerArgs, reply *AddPeerReply) error {
	// 验证输入参数
	if args.Name == "" {
		*reply = AddPeerReply{Success: false, Message: "Peer name cannot be empty", Err: fmt.Errorf("peer name cannot be empty")}
		return nil
	}
	if args.ID == "" {
		*reply = AddPeerReply{Success: false, Message: "Peer ID cannot be empty", Err: fmt.Errorf("peer ID cannot be empty")}
		return nil
	}

	// 解码 peer ID
	peerID, err := peer.Decode(args.ID)
	if err != nil {
		*reply = AddPeerReply{Success: false, Message: fmt.Sprintf("Invalid peer ID format: %v", err), Err: err}
		return nil
	}

	// 检查是否已存在相同 ID 的 peer
	for _, p := range hsr.config.Peers {
		if p.ID == peerID {
			*reply = AddPeerReply{Success: false, Message: fmt.Sprintf("Peer with ID %s already exists", args.ID), Err: fmt.Errorf("peer already exists")}
			return nil
		}
	}

	// 检查是否已存在相同名称的 peer
	for _, p := range hsr.config.Peers {
		if p.Name == args.Name {
			*reply = AddPeerReply{Success: false, Message: fmt.Sprintf("Peer with name %s already exists", args.Name), Err: fmt.Errorf("peer name already exists")}
			return nil
		}
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

// -----------------------------------------------------------------------------------------------------------------------------------------------------}

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

	// On Windows, use TCP instead of Unix socket
	l, err := net.Listen("tcp", "127.0.0.1:0") // Use random available port
	if err != nil {
		log.Fatal("[!] Failed to launch RPC server: ", err)
	}

	addr := l.Addr().(*net.TCPAddr)
	portFile := filepath.Join(os.TempDir(), fmt.Sprintf("mynetwork-rpc.%s.port", config.Interface))
	err = os.WriteFile(portFile, []byte(fmt.Sprintf("%d", addr.Port)), 0644)
	if err != nil {
		log.Printf("[!] Warning: Could not write port file: %s", err)
	}

	fmt.Printf("[-] RPC server ready on %s\n", l.Addr().String())

	// Use a done channel to signal when Accept should stop
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := l.Accept()
			if err != nil {
				// Check if this is due to listener being closed
				select {
				case <-ctx.Done():
					// Expected closure, don't log error
					return
				default:
					// Unexpected error
					log.Printf("[!] RPC Accept error: %v", err)
					return
				}
			}
			go rpc.ServeConn(conn)
		}
	}()

	<-ctx.Done()
	fmt.Println("[-] Closing RPC server")
	l.Close()

	// Wait for Accept goroutine to finish
	<-done
}

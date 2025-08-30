package rpc

import (
	"fmt"
	"log"
	"net/rpc"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func getClient(ifname string) *rpc.Client {
	if runtime.GOOS == "windows" {
		return connect_windows(ifname)
	}
	return connect_linux(ifname)
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func connect_windows(ifname string) *rpc.Client {
	// Try to read port from lock file or use a default approach
	lockPath := filepath.Join(os.TempDir(), fmt.Sprintf("mynetwork-rpc.%s.port", ifname))

	// Try multiple times as the server might still be starting
	for attempts := 0; attempts < 10; attempts++ {
		if data, err := os.ReadFile(lockPath); err == nil {
			portStr := strings.TrimSpace(string(data))
			if port, err := strconv.Atoi(portStr); err == nil {
				client, err := rpc.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
				if err == nil {
					return client
				}
				log.Printf("[!] Failed to connect to RPC server on port %d from port file: %v", port, err)
			}
		}

		// Try common ports if lock file doesn't exist
		for port := 9000; port < 9100; port++ {
			client, err := rpc.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
			if err == nil {
				return client
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	log.Fatal("[!] Failed to connect to RPC server: Could not find Windows TCP RPC server. Please ensure mynetwork service is running with 'mynetwork up' command.")
	return nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func connect_linux(ifname string) *rpc.Client {
	client, err := rpc.Dial("unix", fmt.Sprintf("/run/mynetwork-rpc.%s.sock", ifname))
	if err != nil {
		log.Fatal("[!] Failed to connect to RPC server: ", err)
	}
	return client
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func Status(ifname string) StatusReply {
	client := getClient(ifname)
	var reply StatusReply
	if err := client.Call("HyprspaceRPC.Status", new(Args), &reply); err != nil {
		log.Fatal("[!] RPC call failed: ", err)
	}
	return reply
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func Peers(ifname string) PeersReply {
	client := getClient(ifname)
	var reply PeersReply
	if err := client.Call("HyprspaceRPC.Peers", new(Args), &reply); err != nil {
		log.Fatal("[!] RPC call failed: ", err)
	}
	return reply
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func Route(ifname string, args RouteArgs) RouteReply {
	client := getClient(ifname)
	var reply RouteReply
	if err := client.Call("HyprspaceRPC.Route", args, &reply); err != nil {
		log.Fatal("[!] RPC call failed: ", err)
	}
	return reply
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func AddPeer(ifname string, args AddPeerArgs) AddPeerReply {
	client := getClient(ifname)
	var reply AddPeerReply
	if err := client.Call("HyprspaceRPC.AddPeer", args, &reply); err != nil {
		log.Fatal("[!] RPC call failed: ", err)
	}
	return reply
}

package cli

import (
	"fmt"

	"github.com/DataDrake/cli-ng/v2/cmd"
	"github.com/soitun/mynetwork/rpc"
)

// -----------------------------------------------------------------------------------------------------------------------------------------------------
var Peers = cmd.Sub{
	Name:  "peers",
	Short: "List peer connections",
	Run:   PeersRun,
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func PeersRun(r *cmd.Root, c *cmd.Sub) {
	ifName := r.Flags.(*GlobalFlags).InterfaceName
	if ifName == "" {
		ifName = "mynetwork"
	}

	peers := rpc.Peers(ifName)
	for _, peer := range peers.Peers {
		fmt.Printf("Name: %s, PeerID: %s, IPv4: %s, IPv6: %s\n", peer.Name, peer.PeerID, peer.IPv4, peer.IPv6)
	}
}

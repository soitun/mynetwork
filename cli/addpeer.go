package cli

import (
	"fmt"

	"github.com/DataDrake/cli-ng/v2/cmd"
	"github.com/soitun/mynetwork/rpc"
)

// -----------------------------------------------------------------------------------------------------------------------------------------------------
var AddPeer = cmd.Sub{
	Name:  "addpeer",
	Alias: "ap",
	Short: "Add a new peer dynamically",
	Args:  &AddPeerArgs{},
	Run:   AddPeerRun,
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
type AddPeerArgs struct {
	Name string `desc:"Name of the peer to add"`
	ID   string `desc:"Peer ID to add"`
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func AddPeerRun(r *cmd.Root, c *cmd.Sub) {
	// Parse Command Args
	args := c.Args.(*AddPeerArgs)
	ifName := r.Flags.(*GlobalFlags).InterfaceName
	if ifName == "" {
		ifName = "mynetwork"
	}

	// Validate arguments
	if args.Name == "" {
		fmt.Println("Error: Peer name is required")
		return
	}
	if args.ID == "" {
		fmt.Println("Error: Peer ID is required")
		return
	}

	// Call RPC to add peer
	rpcArgs := rpc.AddPeerArgs{
		Name: args.Name,
		ID:   args.ID,
	}
	reply := rpc.AddPeer(ifName, rpcArgs)

	if reply.Success {
		fmt.Printf("Success: %s\n", reply.Message)
	} else {
		fmt.Printf("Error: %s\n", reply.Message)
	}
}
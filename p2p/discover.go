package p2p

import (
	"context"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/soitun/mynetwork/config"
)

// -----------------------------------------------------------------------------------------------------------------------------------------------------
var discoverNow = make(chan bool)

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// Discover starts up a DHT based discovery system finding and adding nodes with the same rendezvous string.
func Discover(ctx context.Context, h host.Host, dht *dht.IpfsDHT, cfg *config.Config) {
	dur := time.Second * 1
	ticker := time.NewTicker(dur)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-discoverNow:
			dur = time.Second * 3
			// Immediately trigger discovery
			ticker.Reset(time.Millisecond * 1)
		case <-ticker.C:
			connectedToAny := false
			// 检查当前所有的 peers（包括动态添加的）
			for _, p := range cfg.Peers {
				if h.Network().Connectedness(p.ID) != network.Connected {
					_, err := h.Network().DialPeer(ctx, p.ID)
					if err != nil {
						continue
					}
					connectedToAny = true
				} else {
					connectedToAny = true
				}
			}
			if !connectedToAny {
				// fmt.Println("[!] Not connected to any peers, attempting to bootstrap again")
				dht.Bootstrap(ctx)
				dht.RefreshRoutingTable()
				dur = time.Second * 10
				ticker.Reset(dur)
			} else {
				dur = dur * 2
				if dur >= time.Second*60 {
					dur = time.Second * 60
				}
				ticker.Reset(dur)
			}
		}
	}
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func Rediscover() {
	discoverNow <- true
}

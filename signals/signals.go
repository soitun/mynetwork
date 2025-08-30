//go:build !windows
// +build !windows

package signals

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/soitun/mynetwork/p2p"
	"github.com/soitun/mynetwork/tun"
)

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func SignalHandler(ctx context.Context, host host.Host, lockPath string, dht *dht.IpfsDHT, tunDev *tun.TUN, ctxCancel func()) {
	exitCh := make(chan os.Signal, 1)
	rebootstrapCh := make(chan os.Signal, 1)
	signal.Notify(exitCh, syscall.SIGINT, syscall.SIGTERM)
	signal.Notify(rebootstrapCh, syscall.SIGUSR1)

	for {
		select {
		case <-ctx.Done():
			return
		case <-rebootstrapCh:
			fmt.Println("[-] Rebootstrapping on SIGHUP")
			host.ConnManager().TrimOpenConns(context.Background())
			<-dht.ForceRefresh()
			p2p.Rediscover()
		case <-exitCh:
			// Shut the node down
			err := host.Close()
			if err != nil {
				log.Fatal(err)
			}

			// Remove daemon lock from file system.
			err = os.Remove(lockPath)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Println("Received signal, shutting down...")
			tunDev.Iface.Close()
			err = tunDev.Down()
			if err != nil {
				log.Fatal(err)
			}
			ctxCancel()
		}
	}
}

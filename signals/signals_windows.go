//go:build windows
// +build windows

package signals

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/soitun/mynetwork/tun"
	"golang.org/x/sys/windows"
)

// -----------------------------------------------------------------------------------------------------------------------------------------------------
var (
	// Global variables for console control handler
	shutdownCh         chan bool
	handlerSet         bool
	shutdownInProgress bool
)

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// Console control event types
const (
	CTRL_C_EVENT        = 0
	CTRL_BREAK_EVENT    = 1
	CTRL_CLOSE_EVENT    = 2
	CTRL_LOGOFF_EVENT   = 5
	CTRL_SHUTDOWN_EVENT = 6
)

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// SetConsoleCtrlHandler function from kernel32.dll
var (
	kernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procSetConsoleCtrlHandler = kernel32.NewProc("SetConsoleCtrlHandler")
)

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// consoleCtrlHandler handles Windows console control events
func consoleCtrlHandler(ctrlType uintptr) uintptr {
	switch ctrlType {
	case CTRL_C_EVENT, CTRL_BREAK_EVENT, CTRL_CLOSE_EVENT, CTRL_LOGOFF_EVENT, CTRL_SHUTDOWN_EVENT:
		if shutdownInProgress {
			// Shutdown already in progress, ignore additional events
			return 1
		}
		fmt.Printf("[!] Received Windows console control event: %d\n", ctrlType)
		if shutdownCh != nil {
			select {
			case shutdownCh <- true:
				shutdownInProgress = true
			default:
				// Channel is full, shutdown already in progress
			}
		}
		return 1 // TRUE - we handled the event
	default:
		return 0 // FALSE - let default handler process it
	}
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// setupConsoleCtrlHandler sets up Windows-specific console control handler
func setupConsoleCtrlHandler() error {
	if handlerSet {
		return nil
	}

	shutdownCh = make(chan bool, 1)

	// Set the console control handler
	ret, _, err := procSetConsoleCtrlHandler.Call(
		syscall.NewCallback(consoleCtrlHandler),
		1, // TRUE - add handler
	)

	if ret == 0 {
		return fmt.Errorf("failed to set console control handler: %v", err)
	}

	handlerSet = true
	return nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func SignalHandler(ctx context.Context, host host.Host, lockPath string, dht *dht.IpfsDHT, tunDevice *tun.TUN, ctxCancel func()) {
	// Set up both standard Go signal handling and Windows console control handler
	exitCh := make(chan os.Signal, 1)
	signal.Notify(exitCh, syscall.SIGINT, syscall.SIGTERM)

	// Set up Windows-specific console control handler
	err := setupConsoleCtrlHandler()
	if err != nil {
		fmt.Printf("[!] Failed to setup console control handler: %v\n", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-exitCh:
			fmt.Println("[!] Received Go signal, shutting down...")
			performShutdown(host, lockPath, tunDevice, ctxCancel)
			return
		case <-shutdownCh:
			fmt.Println("[!] Received Windows console control event, shutting down...")
			performShutdown(host, lockPath, tunDevice, ctxCancel)
			return
		}
	}
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// performShutdown performs the actual shutdown sequence
func performShutdown(host host.Host, lockPath string, tunDevice *tun.TUN, ctxCancel func()) {
	fmt.Println("[!] Starting graceful shutdown...")

	// Remove console control handler to prevent additional events
	if handlerSet {
		procSetConsoleCtrlHandler.Call(
			0, // NULL - remove all handlers
			0, // FALSE - remove handler
		)
		handlerSet = false
	}

	// Set up a timeout for forced exit
	shutdownTimeout := time.NewTimer(5 * time.Second)
	go func() {
		<-shutdownTimeout.C
		fmt.Println("[!] Shutdown timeout reached, forcing exit")
		os.Exit(1)
	}()

	// Cancel context first to stop all goroutines
	ctxCancel()

	// Give a moment for goroutines to exit gracefully
	time.Sleep(100 * time.Millisecond)

	// Close host connection
	if err := host.Close(); err != nil {
		fmt.Printf("[!] Failed to close host: %v\n", err)
	}

	// Close TUN device
	if tunDevice != nil {
		if tunDevice.Iface != nil {
			tunDevice.Iface.Close()
		}
		if err := tunDevice.Down(); err != nil {
			fmt.Printf("[!] Failed to bring down TUN device: %v\n", err)
		}
	}

	// Remove daemon lock from file system
	if _, err := os.Stat(lockPath); err == nil {
		if err := os.Remove(lockPath); err != nil {
			fmt.Printf("[!] Failed to remove lock file: %v\n", err)
		}
	}

	// Stop the timeout timer since we completed successfully
	shutdownTimeout.Stop()

	fmt.Println("[-] Shutdown complete")
	os.Exit(0)
}

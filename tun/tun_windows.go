//go:build windows
// +build windows

package tun

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os/exec"
	"strings"

	"github.com/soitun/mynetwork/wintun"
	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
)

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// WindowsTUN extends TUN for Windows-specific functionality
type WindowsTUN struct {
	TUN
	wintunIface *WintunInterface
	luid        winipcfg.LUID
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// New creates and returns a new TUN interface for the application.
func New(name string, opts ...Option) (*TUN, error) {
	result := &WindowsTUN{}

	// Create Wintun interface first
	tunDevice, err := wintun.CreateTUN(name, 1420) // Default MTU
	if err != nil {
		return nil, fmt.Errorf("create TUN device failed: %w", err)
	}

	// Create a wrapper that implements water.Interface-like interface
	wintunIface := &WintunInterface{
		device: tunDevice,
		name:   name,
	}

	// Store the wintun interface
	result.wintunIface = wintunIface
	result.Iface = wintunIface

	// Get LUID for winipcfg operations
	result.luid = winipcfg.LUID(tunDevice.LUID())

	// Apply options to set struct values (including IP addresses)
	err = result.Apply(opts...)
	if err != nil {
		return nil, err
	}

	return &result.TUN, err
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// WintunInterface wraps wintun.Device to provide water.Interface-compatible interface
type WintunInterface struct {
	device wintun.Device
	name   string
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// Read implements io.Reader
func (w *WintunInterface) Read(p []byte) (int, error) {
	return w.device.Read(p, 0)
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// Write implements io.Writer
func (w *WintunInterface) Write(p []byte) (int, error) {
	return w.device.Write(p, 0)
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// Close implements io.Closer
func (w *WintunInterface) Close() error {
	return w.device.Close()
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// Name returns the interface name (compatible with water.Interface)
func (w *WintunInterface) Name() string {
	name, _ := w.device.Name()
	return name
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// IsTUN returns true (compatible with water.Interface)
func (w *WintunInterface) IsTUN() bool {
	return true
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// IsTAP returns false (compatible with water.Interface)
func (w *WintunInterface) IsTAP() bool {
	return false
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// setMTU configures the interface's MTU.
func (t *TUN) setMTU(mtu int) error {
	t.MTU = mtu
	return nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// setAddress configures the interface's address.
func (t *TUN) setAddress(address string) error {
	if t.Src == "" {
		t.Src = address // Keep first address as Src for backward compatibility
	}
	t.Addresses = append(t.Addresses, address)
	return nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// setupWintunInterface configures the Wintun interface using netsh commands
func (wt *WindowsTUN) setupWintunInterface() error {
	// Get interface name
	ifaceName := wt.wintunIface.Name()

	// Parse IP address to determine if it's IPv4 or IPv6
	ipStr := wt.Src
	if strings.Contains(ipStr, "/") {
		ipStr = strings.Split(ipStr, "/")[0]
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return fmt.Errorf("invalid IP address: %s", wt.Src)
	}

	// Set interface IP address using appropriate netsh command
	var err error
	if ip.To4() != nil {
		// IPv4 address
		err = netsh("interface", "ipv4", "set", "address", "name="+ifaceName, "static", wt.Src)
	} else {
		// IPv6 address
		err = netsh("interface", "ipv6", "set", "address", "interface="+ifaceName, "address="+wt.Src)
	}
	if err != nil {
		return fmt.Errorf("set IP address failed: %w", err)
	}

	// Set MTU using netsh
	err = netsh("interface", "ipv4", "set", "subinterface", ifaceName, "mtu="+fmt.Sprintf("%d", wt.MTU))
	if err != nil {
		return fmt.Errorf("set MTU failed: %w", err)
	}

	return nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// setupMTU sets the Maximum Tansmission Unit Size for a
// Packet on the interface.
func (t *TUN) setupMTU(mtu int) error {
	return netsh("interface", "ipv4", "set", "subinterface", t.Iface.Name(), "mtu=", fmt.Sprintf("%d", mtu))
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// setupAddress sets the interface's destination address and subnet.
func (t *TUN) setupAddress(address string) error {
	return netsh("interface", "ip", "set", "address", "name=", t.Iface.Name(), "static", address)
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// SetDestAddress isn't supported under Windows.
// You should instead use set address to set the interface to handle
// all addresses within a subnet.
func (t *TUN) setDestAddress(address string) error {
	return errors.New("destination addresses are not supported under windows")
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// Up brings up an interface to allow it to start accepting connections.
func (t *TUN) Up() error {
	// For Windows, we need to setup the Wintun interface
	// Check if this TUN is part of a WindowsTUN by looking for setupWintunInterface capability
	if setupFunc := getWindowsSetupFunc(t); setupFunc != nil {
		return setupFunc()
	}
	return nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// getWindowsSetupFunc tries to get the Windows-specific setup function
func getWindowsSetupFunc(t *TUN) func() error {
	// Check if this TUN has a Wintun interface
	if t.Src != "" && t.Iface != nil {
		return func() error {
			return setupWintunWithWinipcfg(t)
		}
	}
	return nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// setupWintunWithWinipcfg configures the Wintun interface using winipcfg
// This implementation follows the pattern used in Nebula project
func setupWintunWithWinipcfg(t *TUN) error {
	wt, ok := t.Iface.(*WintunInterface)
	if !ok {
		return errors.New("interface is not a WintunInterface")
	}

	// Get the LUID for the interface
	luid := winipcfg.LUID(wt.device.LUID())

	// Process all addresses
	var addresses []netip.Prefix
	for _, address := range t.Addresses {
		address = strings.TrimSpace(address)
		if address == "" {
			continue
		}

		// Split address and prefix
		parts := strings.Split(address, "/")
		if len(parts) != 2 {
			return fmt.Errorf("invalid address format: %s", address)
		}

		ip := net.ParseIP(parts[0])
		if ip == nil {
			return fmt.Errorf("invalid IP address: %s", parts[0])
		}

		// Parse prefix length
		var prefixLen int
		if _, err := fmt.Sscanf(parts[1], "%d", &prefixLen); err != nil {
			return fmt.Errorf("invalid prefix length: %s", parts[1])
		}

		// Create netip.Prefix
		var prefix netip.Prefix
		if ip.To4() != nil {
			// IPv4
			addr, ok := netip.AddrFromSlice(ip.To4())
			if !ok {
				return fmt.Errorf("failed to create IPv4 address")
			}
			prefix = netip.PrefixFrom(addr, prefixLen)
		} else {
			// IPv6
			addr, ok := netip.AddrFromSlice(ip.To16())
			if !ok {
				return fmt.Errorf("failed to create IPv6 address")
			}
			prefix = netip.PrefixFrom(addr, prefixLen)
		}

		addresses = append(addresses, prefix)
	}

	if len(addresses) == 0 {
		return errors.New("no valid addresses specified")
	}

	// Set IP addresses using winipcfg
	err := luid.SetIPAddresses(addresses)
	if err != nil {
		return fmt.Errorf("failed to set IP addresses: %v", err)
	}

	// Set MTU if specified
	if t.MTU > 0 {
		// Get interface for setting MTU
		iface, err := luid.IPInterface(windows.AF_INET)
		if err == nil {
			iface.NLMTU = uint32(t.MTU)
			err = iface.Set()
			if err != nil {
				fmt.Printf("Warning: failed to set IPv4 MTU: %v\n", err)
			}
		}

		// Also try to set IPv6 MTU
		iface6, err := luid.IPInterface(windows.AF_INET6)
		if err == nil {
			iface6.NLMTU = uint32(t.MTU)
			err = iface6.Set()
			if err != nil {
				fmt.Printf("Warning: failed to set IPv6 MTU: %v\n", err)
			}
		}
	}

	return nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// Down brings down an interface stopping active connections.
func (t *TUN) Down() error {
	return nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// Delete removes a TUN device from the host.
func Delete(name string) error {
	return netsh("interface", "set", "interface", "name=", name, "disable")
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// addRoute adds a route to the system routing table
func (t *TUN) addRoute(dest net.IPNet) error {
	// Check if route already exists before adding
	if routeExists(dest, t.Iface.Name()) {
		// Route already exists, skip adding
		return nil
	}

	// Determine if this is IPv4 or IPv6
	if dest.IP.To4() != nil {
		// IPv4 route
		return netsh("interface", "ipv4", "add", "route", dest.String(), t.Iface.Name())
	} else {
		// IPv6 route
		return netsh("interface", "ipv6", "add", "route", dest.String(), t.Iface.Name())
	}
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// delRoute removes a route from the system routing table
func (t *TUN) delRoute(dest net.IPNet) error {
	// Determine if this is IPv4 or IPv6
	if dest.IP.To4() != nil {
		// IPv4 route
		return netsh("interface", "ipv4", "delete", "route", dest.String(), t.Iface.Name())
	} else {
		// IPv6 route
		return netsh("interface", "ipv6", "delete", "route", dest.String(), t.Iface.Name())
	}
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// routeExists checks if a route already exists in the system routing table
func routeExists(dest net.IPNet, ifaceName string) bool {
	var args []string
	if dest.IP.To4() != nil {
		// IPv4 route check
		args = []string{"interface", "ipv4", "show", "route"}
	} else {
		// IPv6 route check
		args = []string{"interface", "ipv6", "show", "route"}
	}

	cmd := exec.Command("netsh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If we can't check, assume route doesn't exist
		return false
	}

	// Check if the destination network and interface name appear in the output
	outputStr := string(output)
	destStr := dest.String()

	// Look for lines containing both the destination and interface name
	lines := strings.Split(outputStr, "\n")
	for _, line := range lines {
		if strings.Contains(line, destStr) && strings.Contains(line, ifaceName) {
			return true
		}
	}

	return false
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func netsh(args ...string) (err error) {
	cmd := exec.Command("netsh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("netsh command failed: %s, output: %s", err, string(output))
	}
	return nil
}

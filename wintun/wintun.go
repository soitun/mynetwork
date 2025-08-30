//go:build windows
// +build windows

package wintun

import (
	"crypto/md5"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// -----------------------------------------------------------------------------------------------------------------------------------------------------
type loggerLevel int

// -----------------------------------------------------------------------------------------------------------------------------------------------------
const (
	logInfo loggerLevel = iota
	logWarn
	logErr
)

// -----------------------------------------------------------------------------------------------------------------------------------------------------
const AdapterNameMax = 128
const tunGUIDLabel = "Fixed MyNetwork Windows GUID v1"

// -----------------------------------------------------------------------------------------------------------------------------------------------------
type Adapter struct {
	handle uintptr
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
type Device interface {
	Read(buff []byte, offset int) (int, error)
	Write(buff []byte, offset int) (int, error)
	Flush() error
	Close() error
	Name() (string, error)
	LUID() uint64
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
type NativeTun struct {
	adapter   *Adapter
	session   uintptr
	readWait  uintptr
	writeWait uintptr
	name      string
	luid      uint64
	mu        sync.RWMutex // Protects session access
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
var (
	modwintun                      = newLazyDLL("wintun.dll", setupLogger)
	procWintunCreateAdapter        = modwintun.NewProc("WintunCreateAdapter")
	procWintunOpenAdapter          = modwintun.NewProc("WintunOpenAdapter")
	procWintunCloseAdapter         = modwintun.NewProc("WintunCloseAdapter")
	procWintunDeleteDriver         = modwintun.NewProc("WintunDeleteDriver")
	procWintunGetAdapterLUID       = modwintun.NewProc("WintunGetAdapterLUID")
	procWintunStartSession         = modwintun.NewProc("WintunStartSession")
	procWintunEndSession           = modwintun.NewProc("WintunEndSession")
	procWintunGetReadWaitEvent     = modwintun.NewProc("WintunGetReadWaitEvent")
	procWintunReceivePacket        = modwintun.NewProc("WintunReceivePacket")
	procWintunReleaseReceivePacket = modwintun.NewProc("WintunReleaseReceivePacket")
	procWintunAllocateSendPacket   = modwintun.NewProc("WintunAllocateSendPacket")
	procWintunSendPacket           = modwintun.NewProc("WintunSendPacket")
)

// -----------------------------------------------------------------------------------------------------------------------------------------------------
type lazyDLL struct {
	name  string
	mu    sync.Mutex
	dll   *windows.DLL
	setup func()
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func newLazyDLL(name string, setup func()) *lazyDLL {
	return &lazyDLL{name: name, setup: setup}
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func (d *lazyDLL) Load() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.dll != nil {
		return nil
	}

	// Check if wintun.dll exists before attempting to load
	if err := d.checkDLLExists(); err != nil {
		return err
	}

	dll, err := windows.LoadDLL(d.name)
	if err != nil {
		return err
	}
	if d.setup != nil {
		d.setup()
	}
	d.dll = dll
	return nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func (d *lazyDLL) checkDLLExists() error {
	// Check in current directory first
	if _, err := os.Stat(d.name); err == nil {
		return nil
	}

	// Check in executable directory
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		dllPath := filepath.Join(execDir, d.name)
		if _, err := os.Stat(dllPath); err == nil {
			return nil
		}
	}

	// Check in system PATH
	if _, err := exec.LookPath(d.name); err == nil {
		return nil
	}

	return fmt.Errorf("wintun.dll not found. Please ensure wintun.dll is in the current directory, executable directory, or system PATH")
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func (d *lazyDLL) NewProc(name string) *lazyProc {
	return &lazyProc{dll: d, name: name}
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
type lazyProc struct {
	dll  *lazyDLL
	name string
	mu   sync.Mutex
	proc *windows.Proc
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func (p *lazyProc) Addr() uintptr {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.proc == nil {
		err := p.dll.Load()
		if err != nil {
			panic(err)
		}
		proc, err := p.dll.dll.FindProc(p.name)
		if err != nil {
			panic(err)
		}
		p.proc = proc
	}
	return p.proc.Addr()
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func setupLogger() {
	// Setup wintun logger if needed
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// generateGUIDByDeviceName generates a deterministic GUID based on device name
func generateGUIDByDeviceName(name string) (*windows.GUID, error) {
	hash := md5.New()
	_, err := hash.Write([]byte(tunGUIDLabel))
	if err != nil {
		return nil, err
	}
	_, err = hash.Write([]byte(name))
	if err != nil {
		return nil, err
	}
	sum := hash.Sum(nil)
	return (*windows.GUID)(unsafe.Pointer(&sum[0])), nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// CreateTUN creates a Wintun interface with the given name
func CreateTUN(ifname string, mtu int) (Device, error) {
	return CreateTUNWithRequestedGUID(ifname, nil, mtu)
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// CreateTUNWithRequestedGUID creates a Wintun interface with the given name and a requested GUID
func CreateTUNWithRequestedGUID(ifname string, requestedGUID *windows.GUID, mtu int) (Device, error) {
	if requestedGUID == nil {
		guid, err := generateGUIDByDeviceName(ifname)
		if err != nil {
			return nil, fmt.Errorf("generate GUID failed: %w", err)
		}
		requestedGUID = guid
	}

	adapter, err := CreateAdapter(ifname, "Mynetwork", requestedGUID)
	if err != nil {
		return nil, fmt.Errorf("create adapter failed: %w", err)
	}

	tun := &NativeTun{
		adapter: adapter,
		name:    ifname,
	}

	// Get adapter LUID
	luid, err := adapter.LUID()
	if err != nil {
		return nil, fmt.Errorf("get adapter LUID failed: %w", err)
	}
	tun.luid = luid

	// Start session
	session, err := adapter.StartSession(0x800000) // 8MB ring buffer
	if err != nil {
		return nil, fmt.Errorf("start session failed: %w", err)
	}
	tun.session = session

	// Get read wait event
	readWait, err := GetReadWaitEvent(session)
	if err != nil {
		return nil, fmt.Errorf("get read wait event failed: %w", err)
	}
	tun.readWait = readWait

	return tun, nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// CreateAdapter creates a Wintun adapter
func CreateAdapter(name string, tunnelType string, requestedGUID *windows.GUID) (*Adapter, error) {
	name16, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	tunnelType16, err := windows.UTF16PtrFromString(tunnelType)
	if err != nil {
		return nil, err
	}
	r0, _, e1 := syscall.Syscall(procWintunCreateAdapter.Addr(), 3,
		uintptr(unsafe.Pointer(name16)),
		uintptr(unsafe.Pointer(tunnelType16)),
		uintptr(unsafe.Pointer(requestedGUID)))
	if r0 == 0 {
		return nil, e1
	}
	adapter := &Adapter{handle: r0}
	runtime.SetFinalizer(adapter, closeAdapter)
	return adapter, nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func closeAdapter(adapter *Adapter) {
	adapter.Close()
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// Close closes the adapter
func (adapter *Adapter) Close() error {
	if adapter.handle == 0 {
		return nil
	}
	_, _, e1 := syscall.Syscall(procWintunCloseAdapter.Addr(), 1, adapter.handle, 0, 0)
	adapter.handle = 0
	if e1 != 0 {
		return e1
	}
	return nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// LUID returns the adapter's LUID
func (adapter *Adapter) LUID() (uint64, error) {
	var luid uint64
	r0, _, e1 := syscall.Syscall(procWintunGetAdapterLUID.Addr(), 2,
		adapter.handle,
		uintptr(unsafe.Pointer(&luid)), 0)
	if r0 == 0 {
		return 0, e1
	}
	return luid, nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// StartSession starts a Wintun session
func (adapter *Adapter) StartSession(capacity uint32) (uintptr, error) {
	r0, _, e1 := syscall.Syscall(procWintunStartSession.Addr(), 2,
		adapter.handle,
		uintptr(capacity), 0)
	if r0 == 0 {
		return 0, e1
	}
	return r0, nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// GetReadWaitEvent gets the read wait event handle
func GetReadWaitEvent(session uintptr) (uintptr, error) {
	r0, _, e1 := syscall.Syscall(procWintunGetReadWaitEvent.Addr(), 1, session, 0, 0)
	if r0 == 0 {
		return 0, e1
	}
	return r0, nil
}

// NativeTun methods

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// Read reads a packet from the TUN device
func (tun *NativeTun) Read(buff []byte, offset int) (int, error) {
	tun.mu.RLock()
	defer tun.mu.RUnlock()

	// Check if session is still valid
	if tun.session == 0 {
		return 0, fmt.Errorf("session closed")
	}

	var packetSize uint32
	r0, _, e1 := syscall.Syscall(procWintunReceivePacket.Addr(), 2,
		tun.session,
		uintptr(unsafe.Pointer(&packetSize)), 0)
	if r0 == 0 {
		if e1 == windows.ERROR_NO_MORE_ITEMS {
			return 0, nil
		}
		return 0, e1
	}

	if int(packetSize) > len(buff)-offset {
		// Release packet and return error
		syscall.Syscall(procWintunReleaseReceivePacket.Addr(), 2, tun.session, r0, 0)
		return 0, fmt.Errorf("packet too large: %d > %d", packetSize, len(buff)-offset)
	}

	// Copy packet data
	packetData := (*[1500]byte)(unsafe.Pointer(r0))[:packetSize:packetSize]
	copy(buff[offset:], packetData)

	// Release packet
	syscall.Syscall(procWintunReleaseReceivePacket.Addr(), 2, tun.session, r0, 0)

	return int(packetSize), nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// Write writes a packet to the TUN device
func (tun *NativeTun) Write(buff []byte, offset int) (int, error) {
	tun.mu.RLock()
	defer tun.mu.RUnlock()

	// Check if session is still valid
	if tun.session == 0 {
		return 0, fmt.Errorf("session closed")
	}

	packetSize := len(buff) - offset
	r0, _, e1 := syscall.Syscall(procWintunAllocateSendPacket.Addr(), 2,
		tun.session,
		uintptr(packetSize), 0)
	if r0 == 0 {
		return 0, e1
	}

	// Copy data to packet
	packetData := (*[1500]byte)(unsafe.Pointer(r0))[:packetSize:packetSize]
	copy(packetData, buff[offset:])

	// Send packet
	syscall.Syscall(procWintunSendPacket.Addr(), 2, tun.session, r0, 0)

	return packetSize, nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// Flush flushes any pending writes
func (tun *NativeTun) Flush() error {
	return nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// Close closes the TUN device
func (tun *NativeTun) Close() error {
	tun.mu.Lock()
	defer tun.mu.Unlock()

	if tun.session != 0 {
		syscall.Syscall(procWintunEndSession.Addr(), 1, tun.session, 0, 0)
		tun.session = 0
	}
	if tun.adapter != nil {
		return tun.adapter.Close()
	}
	return nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// Name returns the interface name
func (tun *NativeTun) Name() (string, error) {
	return tun.name, nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// LUID returns the interface LUID
func (tun *NativeTun) LUID() uint64 {
	return tun.luid
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// RunningVersion returns the running version of the Wintun driver
func (tun *NativeTun) RunningVersion() (version uint32, err error) {
	// This would require additional Wintun API calls
	return 0, fmt.Errorf("not implemented")
}

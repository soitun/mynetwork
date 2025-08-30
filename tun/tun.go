package tun

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// Interface defines the common interface for TUN devices
type Interface interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Close() error
	Name() string
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// TUN is a struct containing the fields necessary
// to configure a system TUN device. Access the
// internal TUN device through TUN.Iface
type TUN struct {
	Iface     Interface
	MTU       int
	Src       string
	Dst       string
	Addresses []string // Multiple IP addresses for the interface
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// Apply configures the specified options for a TUN device.
func (t *TUN) Apply(opts ...Option) error {
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(t); err != nil {
			return err
		}
	}
	return nil
}

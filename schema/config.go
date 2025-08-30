package schema

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// Config represents the configuration structure for Hyprspace
type Config struct {
	ListenAddresses []string          `json:"listenAddresses"`
	PrivateKey      string            `json:"privateKey"`
	Peers           []Peer            `json:"peers"`
	Services        map[string]string `json:"services"`
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// Peer represents a peer configuration
type Peer struct {
	Id     string  `json:"id"`
	Name   string  `json:"name"`
	Routes []Route `json:"routes"`
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// Route represents a network route
type Route struct {
	Net string `json:"net"`
}

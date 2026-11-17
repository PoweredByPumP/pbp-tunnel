package config

// AppConfig is the root JSON structure for full config files
// Type indicates "client" or "server"
type AppConfig struct {
	Type   string            `json:"type"`
	Client *ClientParameters `json:"client,omitempty"`
	Server *ServerParameters `json:"server,omitempty"`
}

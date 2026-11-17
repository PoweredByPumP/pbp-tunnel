package config

// DefaultPort is the SSH port the server listens on and the client connects to.
const DefaultPort = 52135

type Mode string

const (
	ModeClient Mode = "client"
	ModeServer Mode = "server"
)

// AppConfig is the layout of the JSON config file.
type AppConfig struct {
	Type   Mode              `json:"type"`
	Client *ClientParameters `json:"client,omitempty"`
	Server *ServerParameters `json:"server,omitempty"`
}

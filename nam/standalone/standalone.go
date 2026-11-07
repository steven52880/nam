package standalone

import "github.com/steven52880/nam/ports"

// New wires the standalone adapter. The returned *Client is for startup checks
// (VerifyRoot); the *Auth is already part of the returned ports.
func New(cfg Config) (ports.Ports, *Client, error) {
	if err := cfg.Validate(); err != nil {
		return ports.Ports{}, nil, err
	}
	return build(cfg, NewClient(cfg))
}

func build(cfg Config, c *Client) (ports.Ports, *Client, error) {
	p := ports.Ports{
		Channels: channelStore{c: c},
		Options:  optionStore{c: c},
		Logs:     logStore{c: c},
		Auth:     NewAuth(cfg),
		Backend:  backend{c: c, url: cfg.NewAPIURL},
	}
	return p, c, p.Validate()
}

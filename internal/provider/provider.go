package provider

import (
	"context"
)

// Provider is the interface all VPS providers implement.
// Add a new provider by implementing these 12 methods.
type Provider interface {
	Name() string
	AuthCommand() string
	SetupInstructions() string
	Validate(ctx context.Context) error

	// Servers
	CreateServer(ctx context.Context, opts CreateServerOpts) (*Server, error)
	DeleteServer(ctx context.Context, id string) error
	ListServers(ctx context.Context) ([]Server, error)
	GetServer(ctx context.Context, id string) (*Server, error)

	// Metadata
	ListRegions(ctx context.Context) ([]Region, error)
	ListSizes(ctx context.Context) ([]Size, error)
	ListImages(ctx context.Context) ([]Image, error)

	// SSH keys
	CreateSSHKey(ctx context.Context, name string, publicKey []byte) (*SSHKey, error)
}

type CreateServerOpts struct {
	Name     string
	Region   string
	Size     string
	Image    string
	SSHKeyIDs []string
}

type Server struct {
	ID         string
	Name       string
	Status     string
	PublicIPv4 string
	PublicIPv6 string
	Size       string
	Region     string
	CreatedAt  string
}

type Region struct {
	ID   string
	Name string
	City string
}

type Size struct {
	ID          string
	Name        string
	Description string
	VCPUs       int
	MemoryGB    float64
	DiskGB      int
	MonthlyPrice float64
}

type Image struct {
	ID   string
	Name string
	OS   string
}

type SSHKey struct {
	ID        string
	Name      string
	PublicKey string
}

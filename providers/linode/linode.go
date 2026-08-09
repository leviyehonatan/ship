package linode

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/leviyehonatan/ship/internal/provider"
)

type Provider struct{}

func New() *Provider                                              { return &Provider{} }
func (p *Provider) Name() string                                  { return "linode" }
func (p *Provider) AuthCommand() string                           { return "linode-cli linodes list" }
func (p *Provider) SetupInstructions() string                      { return "linode-cli not installed.\n  Install: https://github.com/linode/linode-cli\nThen: linode-cli configure" }
func (p *Provider) Validate(ctx context.Context) error             { return exec.CommandContext(ctx, "linode-cli", "linodes", "list").Run() }

func (p *Provider) CreateServer(ctx context.Context, opts provider.CreateServerOpts) (*provider.Server, error) {
	out, err := exec.CommandContext(ctx, "linode-cli", "linodes", "create",
		"--label", opts.Name, "--type", opts.Size, "--region", opts.Region,
		"--image", opts.Image, "--root_pass", "ship-"+opts.Name, "--json").Output()
	if err != nil {
		return nil, err
	}
	var s lServer
	json.Unmarshal(out, &s)
	ps := toServer(s)
	return &ps, nil
}

func (p *Provider) DeleteServer(ctx context.Context, id string) error  { return exec.CommandContext(ctx, "linode-cli", "linodes", "delete", id).Run() }

func (p *Provider) ListServers(ctx context.Context) ([]provider.Server, error) {
	out, err := exec.CommandContext(ctx, "linode-cli", "linodes", "list", "--json").Output()
	if err != nil {
		return nil, err
	}
	var data []lServer
	json.Unmarshal(out, &data)
	servers := make([]provider.Server, 0, len(data))
	for _, s := range data {
		servers = append(servers, toServer(s))
	}
	return servers, nil
}

func (p *Provider) GetServer(ctx context.Context, id string) (*provider.Server, error) {
	out, err := exec.CommandContext(ctx, "linode-cli", "linodes", "view", id, "--json").Output()
	if err != nil {
		return nil, err
	}
	var s lServer
	json.Unmarshal(out, &s)
	ps := toServer(s)
	return &ps, nil
}

func (p *Provider) ListRegions(ctx context.Context) ([]provider.Region, error) {
	out, err := exec.CommandContext(ctx, "linode-cli", "regions", "list", "--json").Output()
	if err != nil {
		return nil, err
	}
	var regions []struct{ ID, Country string }
	json.Unmarshal(out, &regions)
	result := make([]provider.Region, 0, len(regions))
	for _, r := range regions {
		result = append(result, provider.Region{ID: r.ID, Name: r.ID, City: r.Country})
	}
	return result, nil
}

func (p *Provider) ListSizes(ctx context.Context) ([]provider.Size, error) {
	out, err := exec.CommandContext(ctx, "linode-cli", "linodes", "types", "--json").Output()
	if err != nil {
		return nil, err
	}
	var types []struct {
		ID, Label string
		VCPUs     int
		Memory    int
		Disk      int
		Price     struct{ Monthly float64 } `json:"price"`
	}
	json.Unmarshal(out, &types)
	result := make([]provider.Size, 0, len(types))
	for _, t := range types {
		result = append(result, provider.Size{
			ID: t.ID, Name: t.Label, VCPUs: t.VCPUs,
			MemoryGB: float64(t.Memory) / 1024, DiskGB: t.Disk / 1024,
			MonthlyPrice: t.Price.Monthly,
		})
	}
	return result, nil
}

func (p *Provider) ListImages(ctx context.Context) ([]provider.Image, error) {
	out, err := exec.CommandContext(ctx, "linode-cli", "images", "list", "--json").Output()
	if err != nil {
		return nil, err
	}
	var imgs []struct{ ID, Label string }
	json.Unmarshal(out, &imgs)
	result := make([]provider.Image, 0, len(imgs))
	for _, img := range imgs {
		result = append(result, provider.Image{ID: img.ID, Name: img.Label})
	}
	return result, nil
}

func (p *Provider) CreateSSHKey(ctx context.Context, name string, key []byte) (*provider.SSHKey, error) {
	out, err := exec.CommandContext(ctx, "linode-cli", "sshkeys", "create", "--label", name, "--ssh_key", string(key), "--json").Output()
	if err != nil {
		return nil, err
	}
	var k struct{ ID int; Label string }
	json.Unmarshal(out, &k)
	return &provider.SSHKey{ID: fmt.Sprintf("%d", k.ID), Name: k.Label}, nil
}

func (p *Provider) ResizeServer(ctx context.Context, id, size string) error   { return exec.CommandContext(ctx, "linode-cli", "linodes", "resize", id, "--type", size).Run() }
func (p *Provider) ShutdownServer(ctx context.Context, id string) error       { return exec.CommandContext(ctx, "linode-cli", "linodes", "shutdown", id).Run() }
func (p *Provider) PowerOnServer(ctx context.Context, id string) error         { return exec.CommandContext(ctx, "linode-cli", "linodes", "boot", id).Run() }

type lServer struct{ ID int; Label, Status string; IPv4 []string; Type, Region string }

func toServer(s lServer) provider.Server {
	ip := ""
	if len(s.IPv4) > 0 {
		ip = s.IPv4[0]
	}
	return provider.Server{ID: fmt.Sprintf("%d", s.ID), Name: s.Label, Status: s.Status, PublicIPv4: ip, Size: s.Type, Region: s.Region}
}

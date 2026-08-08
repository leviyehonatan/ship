package digitalocean

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/leviyehonatan/ship/internal/provider"
)

type Provider struct{}

func New() *Provider                                    { return &Provider{} }
func (p *Provider) Name() string                        { return "digitalocean" }
func (p *Provider) AuthCommand() string                 { return "doctl account get" }
func (p *Provider) SetupInstructions() string            { return "Install: brew install doctl\nThen: doctl auth init" }
func (p *Provider) Validate(ctx context.Context) error   { return exec.CommandContext(ctx, "doctl", "account", "get").Run() }

func (p *Provider) CreateServer(ctx context.Context, opts provider.CreateServerOpts) (*provider.Server, error) {
	args := []string{"compute", "droplet", "create", opts.Name,
		"--size", opts.Size, "--region", opts.Region, "--image", opts.Image,
		"--output", "json"}
	out, err := exec.CommandContext(ctx, "doctl", args...).Output()
	if err != nil {
		return nil, err
	}
	var s doServer
	json.Unmarshal(out, &s)
	ps := toServer(s)
	return &ps, nil
}

func (p *Provider) DeleteServer(ctx context.Context, id string) error {
	return exec.CommandContext(ctx, "doctl", "compute", "droplet", "delete", id, "-f").Run()
}

func (p *Provider) ListServers(ctx context.Context) ([]provider.Server, error) {
	out, err := exec.CommandContext(ctx, "doctl", "compute", "droplet", "list", "--output", "json").Output()
	if err != nil {
		return nil, err
	}
	var droplets []doServer
	json.Unmarshal(out, &droplets)
	servers := make([]provider.Server, 0, len(droplets))
	for _, d := range droplets {
		servers = append(servers, toServer(d))
	}
	return servers, nil
}

func (p *Provider) GetServer(ctx context.Context, id string) (*provider.Server, error) {
	out, err := exec.CommandContext(ctx, "doctl", "compute", "droplet", "get", id, "--output", "json").Output()
	if err != nil {
		return nil, err
	}
	var s doServer
	json.Unmarshal(out, &s)
	ps := toServer(s)
	return &ps, nil
}

func (p *Provider) ListRegions(ctx context.Context) ([]provider.Region, error) {
	out, err := exec.CommandContext(ctx, "doctl", "compute", "region", "list", "--output", "json").Output()
	if err != nil {
		return nil, err
	}
	var regions []struct{ Slug, Name string }
	json.Unmarshal(out, &regions)
	result := make([]provider.Region, 0, len(regions))
	for _, r := range regions {
		result = append(result, provider.Region{ID: r.Slug, Name: r.Slug, City: r.Name})
	}
	return result, nil
}

func (p *Provider) ListSizes(ctx context.Context) ([]provider.Size, error) {
	out, err := exec.CommandContext(ctx, "doctl", "compute", "size", "list", "--output", "json").Output()
	if err != nil {
		return nil, err
	}
	var sizes []struct{ Slug string; Vcpus int; Memory int; Disk int; PriceMonthly float64 }
	json.Unmarshal(out, &sizes)
	result := make([]provider.Size, 0, len(sizes))
	for _, s := range sizes {
		result = append(result, provider.Size{
			ID: s.Slug, Name: s.Slug, VCPUs: s.Vcpus,
			MemoryGB: float64(s.Memory) / 1024, DiskGB: s.Disk,
			MonthlyPrice: s.PriceMonthly,
		})
	}
	return result, nil
}

func (p *Provider) ListImages(ctx context.Context) ([]provider.Image, error) {
	out, err := exec.CommandContext(ctx, "doctl", "compute", "image", "list", "--public", "--output", "json").Output()
	if err != nil {
		return nil, err
	}
	var imgs []struct{ ID int; Name, Distribution string }
	json.Unmarshal(out, &imgs)
	result := make([]provider.Image, 0, len(imgs))
	for _, img := range imgs {
		result = append(result, provider.Image{ID: fmt.Sprintf("%d", img.ID), Name: img.Distribution + " " + img.Name})
	}
	return result, nil
}

func (p *Provider) CreateSSHKey(ctx context.Context, name string, key []byte) (*provider.SSHKey, error) {
	out, err := exec.CommandContext(ctx, "doctl", "compute", "ssh-key", "create", name, "--public-key", string(key), "--output", "json").Output()
	if err != nil {
		return nil, err
	}
	var k struct{ ID int; Name string }
	json.Unmarshal(out, &k)
	return &provider.SSHKey{ID: fmt.Sprintf("%d", k.ID), Name: k.Name}, nil
}

func (p *Provider) ResizeServer(ctx context.Context, id, size string) error {
	return exec.CommandContext(ctx, "doctl", "compute", "droplet-action", "resize", id, "--size", size, "--wait").Run()
}
func (p *Provider) ShutdownServer(ctx context.Context, id string) error {
	return exec.CommandContext(ctx, "doctl", "compute", "droplet-action", "shutdown", id, "--wait").Run()
}
func (p *Provider) PowerOnServer(ctx context.Context, id string) error {
	return exec.CommandContext(ctx, "doctl", "compute", "droplet-action", "power-on", id, "--wait").Run()
}

type doServer struct{ ID int; Name, Status string; Networks struct{ V4 []struct{ IPAddress string } } `json:"networks"`; SizeSlug, RegionSlug string }

func toServer(s doServer) provider.Server {
	ip := ""
	if len(s.Networks.V4) > 0 {
		ip = s.Networks.V4[0].IPAddress
	}
	return provider.Server{ID: fmt.Sprintf("%d", s.ID), Name: s.Name, Status: s.Status, PublicIPv4: ip, Size: s.SizeSlug, Region: s.RegionSlug}
}

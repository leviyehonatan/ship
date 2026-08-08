package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/leviyehonatan/ship/internal/provider"
)

type Provider struct{}

func New() *Provider                                    { return &Provider{} }
func (p *Provider) Name() string                        { return "gcp" }
func (p *Provider) AuthCommand() string                 { return "gcloud compute instances list" }
func (p *Provider) SetupInstructions() string            { return "Install: brew install google-cloud-sdk\nThen: gcloud auth login && gcloud config set project <project-id>" }
func (p *Provider) Validate(ctx context.Context) error   { return exec.CommandContext(ctx, "gcloud", "compute", "instances", "list", "--limit", "1").Run() }

func (p *Provider) CreateServer(ctx context.Context, opts provider.CreateServerOpts) (*provider.Server, error) {
	out, err := exec.CommandContext(ctx, "gcloud", "compute", "instances", "create", opts.Name,
		"--zone", opts.Region, "--machine-type", opts.Size,
		"--image-family", opts.Image, "--image-project", "ubuntu-os-cloud",
		"--format", "json").Output()
	if err != nil {
		return nil, err
	}
	var instances []struct{ Name, Status, Zone string; MachineType string; NetworkInterfaces []struct{ NetworkIP string } `json:"networkInterfaces"` }
	json.Unmarshal(out, &instances)
	if len(instances) == 0 {
		return nil, fmt.Errorf("instance not created")
	}
	zone := instances[0].Zone[strings.LastIndex(instances[0].Zone, "/")+1:]
	return &provider.Server{ID: instances[0].Name, Name: instances[0].Name, Status: instances[0].Status, Region: zone, Size: instances[0].MachineType}, nil
}

func (p *Provider) DeleteServer(ctx context.Context, id string) error {
	return exec.CommandContext(ctx, "gcloud", "compute", "instances", "delete", id, "--quiet").Run()
}

func (p *Provider) ListServers(ctx context.Context) ([]provider.Server, error) {
	out, err := exec.CommandContext(ctx, "gcloud", "compute", "instances", "list", "--format", "json").Output()
	if err != nil {
		return nil, err
	}
	var instances []struct{ Name, Status, Zone, MachineType string }
	json.Unmarshal(out, &instances)
	servers := make([]provider.Server, 0, len(instances))
	for _, i := range instances {
		zone := i.Zone[strings.LastIndex(i.Zone, "/")+1:]
		servers = append(servers, provider.Server{ID: i.Name, Name: i.Name, Status: i.Status, Region: zone, Size: i.MachineType})
	}
	return servers, nil
}

func (p *Provider) GetServer(ctx context.Context, id string) (*provider.Server, error) {
	out, err := exec.CommandContext(ctx, "gcloud", "compute", "instances", "describe", id, "--format", "json").Output()
	if err != nil {
		return nil, err
	}
	var i struct{ Name, Status, Zone, MachineType string }
	json.Unmarshal(out, &i)
	zone := i.Zone[strings.LastIndex(i.Zone, "/")+1:]
	return &provider.Server{ID: i.Name, Name: i.Name, Status: i.Status, Region: zone, Size: i.MachineType}, nil
}

func (p *Provider) ListRegions(ctx context.Context) ([]provider.Region, error) {
	out, err := exec.CommandContext(ctx, "gcloud", "compute", "zones", "list", "--format", "json").Output()
	if err != nil {
		return nil, err
	}
	var zones []struct{ Name, Region string }
	json.Unmarshal(out, &zones)
	regions := make([]provider.Region, 0, len(zones))
	for _, z := range zones {
		regions = append(regions, provider.Region{ID: z.Name, Name: z.Name, City: z.Region})
	}
	return regions, nil
}

func (p *Provider) ListSizes(ctx context.Context) ([]provider.Size, error) {
	out, err := exec.CommandContext(ctx, "gcloud", "compute", "machine-types", "list", "--format", "json", "--limit", "50").Output()
	if err != nil {
		return nil, err
	}
	var types []struct{ Name, Zone string; GuestCpus int; MemoryMb int }
	json.Unmarshal(out, &types)
	sizes := make([]provider.Size, 0, len(types))
	for _, t := range types {
		sizes = append(sizes, provider.Size{ID: t.Name, Name: t.Name, VCPUs: t.GuestCpus, MemoryGB: float64(t.MemoryMb) / 1024})
	}
	return sizes, nil
}

func (p *Provider) ListImages(ctx context.Context) ([]provider.Image, error) {
	out, err := exec.CommandContext(ctx, "gcloud", "compute", "images", "list",
		"--filter", "family=ubuntu-2404-lts", "--format", "json").Output()
	if err != nil {
		return nil, err
	}
	var imgs []struct{ Name, Family string }
	json.Unmarshal(out, &imgs)
	result := make([]provider.Image, 0, len(imgs))
	for _, i := range imgs {
		result = append(result, provider.Image{ID: i.Name, Name: i.Family, OS: "ubuntu"})
	}
	return result, nil
}

func (p *Provider) CreateSSHKey(ctx context.Context, name string, key []byte) (*provider.SSHKey, error) {
	// gcloud uses project-level SSH keys, not a separate resource
	return &provider.SSHKey{ID: name, Name: name}, nil
}

func (p *Provider) ResizeServer(ctx context.Context, id, size string) error {
	exec.CommandContext(ctx, "gcloud", "compute", "instances", "stop", id, "--quiet").Run()
	return exec.CommandContext(ctx, "gcloud", "compute", "instances", "set-machine-type", id, "--machine-type", size, "--quiet").Run()
}
func (p *Provider) ShutdownServer(ctx context.Context, id string) error { return exec.CommandContext(ctx, "gcloud", "compute", "instances", "stop", id, "--quiet").Run() }
func (p *Provider) PowerOnServer(ctx context.Context, id string) error  { return exec.CommandContext(ctx, "gcloud", "compute", "instances", "start", id, "--quiet").Run() }

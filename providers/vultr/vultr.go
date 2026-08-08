package vultr

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/leviyehonatan/ship/internal/provider"
)

type Provider struct{}

func New() *Provider                                    { return &Provider{} }
func (p *Provider) Name() string                        { return "vultr" }
func (p *Provider) AuthCommand() string                 { return "vultr-cli instance list" }
func (p *Provider) SetupInstructions() string            { return "Install: brew install vultr-cli\nThen: export VULTR_API_KEY=<key>" }
func (p *Provider) Validate(ctx context.Context) error   { return exec.CommandContext(ctx, "vultr-cli", "instance", "list").Run() }

func (p *Provider) CreateServer(ctx context.Context, opts provider.CreateServerOpts) (*provider.Server, error) {
	out, err := exec.CommandContext(ctx, "vultr-cli", "instance", "create",
		"--label", opts.Name, "--plan", opts.Size, "--region", opts.Region,
		"--os", opts.Image, "-o", "json").Output()
	if err != nil {
		return nil, err
	}
	var s struct{ Instance struct{ ID, Label, Status, MainIP, Plan, Region string } `json:"instance"` }
	json.Unmarshal(out, &s)
	return &provider.Server{ID: s.Instance.ID, Name: s.Instance.Label, Status: s.Instance.Status, PublicIPv4: s.Instance.MainIP, Size: s.Instance.Plan, Region: s.Instance.Region}, nil
}

func (p *Provider) DeleteServer(ctx context.Context, id string) error {
	return exec.CommandContext(ctx, "vultr-cli", "instance", "delete", id).Run()
}

func (p *Provider) ListServers(ctx context.Context) ([]provider.Server, error) {
	out, err := exec.CommandContext(ctx, "vultr-cli", "instance", "list", "-o", "json").Output()
	if err != nil {
		return nil, err
	}
	var data struct{ Instances []struct{ ID, Label, Status, MainIP, Plan, Region string } `json:"instances"` }
	json.Unmarshal(out, &data)
	servers := make([]provider.Server, 0, len(data.Instances))
	for _, s := range data.Instances {
		servers = append(servers, provider.Server{ID: s.ID, Name: s.Label, Status: s.Status, PublicIPv4: s.MainIP, Size: s.Plan, Region: s.Region})
	}
	return servers, nil
}

func (p *Provider) GetServer(ctx context.Context, id string) (*provider.Server, error) {
	out, err := exec.CommandContext(ctx, "vultr-cli", "instance", "get", id, "-o", "json").Output()
	if err != nil {
		return nil, err
	}
	var s struct{ Instance struct{ ID, Label, Status, MainIP, Plan, Region string } `json:"instance"` }
	json.Unmarshal(out, &s)
	return &provider.Server{ID: s.Instance.ID, Name: s.Instance.Label, Status: s.Instance.Status, PublicIPv4: s.Instance.MainIP, Size: s.Instance.Plan, Region: s.Instance.Region}, nil
}

func (p *Provider) ListRegions(ctx context.Context) ([]provider.Region, error) {
	out, err := exec.CommandContext(ctx, "vultr-cli", "region", "list", "-o", "json").Output()
	if err != nil {
		return nil, err
	}
	var data struct{ Regions []struct{ ID, City, Country string } `json:"regions"` }
	json.Unmarshal(out, &data)
	result := make([]provider.Region, 0, len(data.Regions))
	for _, r := range data.Regions {
		result = append(result, provider.Region{ID: r.ID, Name: r.ID, City: r.City + ", " + r.Country})
	}
	return result, nil
}

func (p *Provider) ListSizes(ctx context.Context) ([]provider.Size, error) {
	out, err := exec.CommandContext(ctx, "vultr-cli", "plan", "list", "-o", "json").Output()
	if err != nil {
		return nil, err
	}
	var data struct{ Plans []struct{ ID, VCPUCount string; RAM, Disk int; MonthlyCost float64 } `json:"plans"` }
	json.Unmarshal(out, &data)
	result := make([]provider.Size, 0, len(data.Plans))
	for _, p := range data.Plans {
		vCPUs := 1
		fmt.Sscanf(p.VCPUCount, "%d", &vCPUs)
		result = append(result, provider.Size{
			ID: p.ID, Name: p.ID, VCPUs: vCPUs,
			MemoryGB: float64(p.RAM) / 1024, DiskGB: p.Disk,
			MonthlyPrice: p.MonthlyCost,
		})
	}
	return result, nil
}

func (p *Provider) ListImages(ctx context.Context) ([]provider.Image, error) {
	out, err := exec.CommandContext(ctx, "vultr-cli", "os", "list", "-o", "json").Output()
	if err != nil {
		return nil, err
	}
	var data struct{ OSs []struct{ ID int; Name, Family string } `json:"os"` }
	json.Unmarshal(out, &data)
	result := make([]provider.Image, 0, len(data.OSs))
	for _, os := range data.OSs {
		result = append(result, provider.Image{ID: fmt.Sprintf("%d", os.ID), Name: os.Name, OS: os.Family})
	}
	return result, nil
}

func (p *Provider) CreateSSHKey(ctx context.Context, name string, key []byte) (*provider.SSHKey, error) {
	out, err := exec.CommandContext(ctx, "vultr-cli", "ssh-key", "create", "--name", name, "--key", string(key), "-o", "json").Output()
	if err != nil {
		return nil, err
	}
	var k struct{ SSHKey struct{ ID, Name string } `json:"ssh_key"` }
	json.Unmarshal(out, &k)
	return &provider.SSHKey{ID: k.SSHKey.ID, Name: k.SSHKey.Name}, nil
}

func (p *Provider) ResizeServer(ctx context.Context, id, size string) error {
	return exec.CommandContext(ctx, "vultr-cli", "instance", "upgrade-plan", id, "--plan", size).Run()
}
func (p *Provider) ShutdownServer(ctx context.Context, id string) error { return exec.CommandContext(ctx, "vultr-cli", "instance", "stop", id).Run() }
func (p *Provider) PowerOnServer(ctx context.Context, id string) error   { return exec.CommandContext(ctx, "vultr-cli", "instance", "start", id).Run() }

package hetzner

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"

	"github.com/leviyehonatan/ship/internal/provider"
)

type Provider struct{}

func New() *Provider {
	return &Provider{}
}

func (p *Provider) Name() string { return "hetzner" }

func (p *Provider) AuthCommand() string {
	return "hcloud server list"
}

func (p *Provider) SetupInstructions() string {
	return `hcloud is not authenticated.

  Install the CLI:
    brew install hcloud

  Then authenticate:
    hcloud context create my-project
    # paste your API token from https://console.hetzner.cloud/ → Security → API Tokens

  Or run:
    hcloud server list   (will prompt for token on first run)`
}

func (p *Provider) Validate(ctx context.Context) error {
	return exec.CommandContext(ctx, "hcloud", "server", "list").Run()
}

func (p *Provider) CreateServer(ctx context.Context, opts provider.CreateServerOpts) (*provider.Server, error) {
	args := []string{
		"server", "create",
		"--name", opts.Name,
		"--type", opts.Size,
		"--image", opts.Image,
		"--location", opts.Region,
		"--output", "json",
	}
	for _, kid := range opts.SSHKeyIDs {
		args = append(args, "--ssh-key", kid)
	}

	out, err := exec.CommandContext(ctx, "hcloud", args...).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("hcloud: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("hcloud: %w", err)
	}

	var result hcloudCreateResponse
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("parse hcloud output: %w", err)
	}

	return &provider.Server{
		ID:         fmt.Sprintf("%d", result.Server.ID),
		Name:       result.Server.Name,
		Status:     result.Server.Status,
		PublicIPv4: result.Server.PublicNet.IPv4.IP,
		PublicIPv6: result.Server.PublicNet.IPv6.IP,
		Size:       result.Server.ServerType.Name,
		Region:     result.Server.Location.Name,
		CreatedAt:  result.Server.Created,
	}, nil
}

func (p *Provider) DeleteServer(ctx context.Context, id string) error {
	cmd := exec.CommandContext(ctx, "hcloud", "server", "delete", id)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("hcloud delete: %s", string(out))
	}
	return nil
}

func (p *Provider) ListServers(ctx context.Context) ([]provider.Server, error) {
	out, err := exec.CommandContext(ctx, "hcloud", "server", "list", "--output", "json").Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("hcloud: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("hcloud: %w", err)
	}

	var servers []hcloudServer
	if err := json.Unmarshal(out, &servers); err != nil {
		return nil, fmt.Errorf("parse hcloud output: %w\n%s", err, string(out))
	}

	result := make([]provider.Server, 0, len(servers))
	for _, s := range servers {
		result = append(result, provider.Server{
			ID:         fmt.Sprintf("%d", s.ID),
			Name:       s.Name,
			Status:     s.Status,
			PublicIPv4: s.PublicNet.IPv4.IP,
			PublicIPv6: s.PublicNet.IPv6.IP,
			Size:       s.ServerType.Name,
			Region:     s.Location.Name,
			CreatedAt:  s.Created,
		})
	}
	return result, nil
}

func (p *Provider) GetServer(ctx context.Context, id string) (*provider.Server, error) {
	out, err := exec.CommandContext(ctx, "hcloud", "server", "describe", "--output", "json", id).Output()
	if err != nil {
		return nil, err
	}
	var s hcloudServer
	if err := json.Unmarshal(out, &s); err != nil {
		return nil, err
	}
	return &provider.Server{
		ID:         fmt.Sprintf("%d", s.ID),
		Name:       s.Name,
		Status:     s.Status,
		PublicIPv4: s.PublicNet.IPv4.IP,
		PublicIPv6: s.PublicNet.IPv6.IP,
		Size:       s.ServerType.Name,
		Region:     s.Location.Name,
		CreatedAt:  s.Created,
	}, nil
}

func (p *Provider) ListRegions(ctx context.Context) ([]provider.Region, error) {
	out, err := exec.CommandContext(ctx, "hcloud", "location", "list", "--output", "json").Output()
	if err != nil {
		return nil, err
	}
	var locs []hcloudLocation
	if err := json.Unmarshal(out, &locs); err != nil {
		return nil, err
	}
	result := make([]provider.Region, 0, len(locs))
	for _, l := range locs {
		result = append(result, provider.Region{
			ID:   l.Name,
			Name: l.Name,
			City: l.City,
		})
	}
	return result, nil
}

func (p *Provider) ListSizes(ctx context.Context) ([]provider.Size, error) {
	out, err := exec.CommandContext(ctx, "hcloud", "server-type", "list", "--output", "json").Output()
	if err != nil {
		return nil, err
	}
	var types []hcloudServerType
	if err := json.Unmarshal(out, &types); err != nil {
		return nil, err
	}
	result := make([]provider.Size, 0, len(types))
	for _, t := range types {
		if t.Architecture != "x86" && t.Architecture != "arm" {
			continue
		}
		var monthly float64
		for _, pr := range t.Prices {
			if pr.PriceMonthly.Gross == "" {
				continue
			}
			val, _ := strconv.ParseFloat(pr.PriceMonthly.Gross, 64)
			if val <= 0 {
				continue
			}
			// Prefer European locations (skip US/Asia for display pricing)
			if isEULocation(pr.Location) {
				monthly = val
				break
			}
		}
		// Fallback: take first with a price
		if monthly == 0 {
			for _, pr := range t.Prices {
				if pr.PriceMonthly.Gross != "" {
					fmt.Sscanf(pr.PriceMonthly.Gross, "%f", &monthly)
					if monthly > 0 {
						break
					}
				}
			}
		}
		result = append(result, provider.Size{
			ID:           t.Name,
			Name:         t.Name,
			Description:  t.Description,
			VCPUs:        t.Cores,
			MemoryGB:     t.Memory,
			DiskGB:       t.Disk,
			MonthlyPrice: monthly,
		})
	}
	return result, nil
}

func (p *Provider) ListImages(ctx context.Context) ([]provider.Image, error) {
	out, err := exec.CommandContext(ctx, "hcloud", "image", "list", "--type", "system", "--output", "json").Output()
	if err != nil {
		return nil, err
	}
	var imgs []hcloudImage
	if err := json.Unmarshal(out, &imgs); err != nil {
		return nil, err
	}
	result := make([]provider.Image, 0, len(imgs))
	for _, img := range imgs {
		result = append(result, provider.Image{
			ID:   fmt.Sprintf("%d", img.ID),
			Name: img.Description,
			OS:   img.OSFlavor,
		})
	}
	return result, nil
}

func (p *Provider) CreateSSHKey(ctx context.Context, name string, publicKey []byte) (*provider.SSHKey, error) {
	// hcloud ssh-key create --name <name> --public-key "<key>" --output json
	cmd := exec.CommandContext(ctx, "hcloud", "ssh-key", "create",
		"--name", name,
		"--public-key", string(publicKey),
		"--output", "json",
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("creating ssh key: %w", err)
	}
	var key hcloudSSHKey
	if err := json.Unmarshal(out, &key); err != nil {
		return nil, err
	}
	return &provider.SSHKey{
		ID:        fmt.Sprintf("%d", key.ID),
		Name:      key.Name,
		PublicKey: key.PublicKey,
	}, nil
}

func (p *Provider) ResizeServer(ctx context.Context, serverID string, newSize string) error {
	cmd := exec.CommandContext(ctx, "hcloud", "server", "change-type", serverID, "--type", newSize)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("resize: %w\n%s", err, out)
	}
	return nil
}

func (p *Provider) ShutdownServer(ctx context.Context, serverID string) error {
	return exec.CommandContext(ctx, "hcloud", "server", "poweroff", serverID).Run()
}

func (p *Provider) PowerOnServer(ctx context.Context, serverID string) error {
	return exec.CommandContext(ctx, "hcloud", "server", "poweron", serverID).Run()
}

// JSON response types matching hcloud --output json

type hcloudCreateResponse struct {
	Server hcloudServer `json:"server"`
}

type hcloudServer struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Created    string `json:"created"`
	ServerType struct {
		Name string `json:"name"`
	} `json:"server_type"`
	Location struct {
		Name string `json:"name"`
	} `json:"location"`
	PublicNet struct {
		IPv4 struct {
			IP string `json:"ip"`
		} `json:"ipv4"`
		IPv6 struct {
			IP string `json:"ip"`
		} `json:"ipv6"`
	} `json:"public_net"`
}

type hcloudLocation struct {
	Name string `json:"name"`
	City string `json:"city"`
}

type hcloudServerType struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	Cores        int     `json:"cores"`
	Memory       float64 `json:"memory"`
	Disk         int     `json:"disk"`
	Architecture string  `json:"architecture"`
	Prices       []struct {
		Location     string `json:"location"`
		PriceMonthly struct {
			Gross string `json:"gross"`
		} `json:"price_monthly"`
	} `json:"prices"`
}

type hcloudImage struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
	OSFlavor    string `json:"os_flavor"`
}

type hcloudSSHKey struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
}

func isEULocation(loc string) bool {
	euLocs := map[string]bool{
		"fsn1": true, "nbg1": true, "hel1": true,
	}
	return euLocs[loc]
}

package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/leviyehonatan/ship/internal/provider"
)

type Provider struct{}

func New() *Provider                                    { return &Provider{} }
func (p *Provider) Name() string                        { return "aws" }
func (p *Provider) AuthCommand() string                 { return "aws ec2 describe-instances" }
func (p *Provider) SetupInstructions() string            { return "aws CLI not installed.\n  Install: https://aws.amazon.com/cli/\nThen: aws configure" }
func (p *Provider) Validate(ctx context.Context) error   { return exec.CommandContext(ctx, "aws", "ec2", "describe-instances", "--max-items", "1").Run() }

func (p *Provider) CreateServer(ctx context.Context, opts provider.CreateServerOpts) (*provider.Server, error) {
	out, err := exec.CommandContext(ctx, "aws", "ec2", "run-instances",
		"--image-id", opts.Image, "--instance-type", opts.Size,
		"--region", opts.Region, "--output", "json",
		"--tag-specifications", fmt.Sprintf("ResourceType=instance,Tags=[{Key=Name,Value=%s}]", opts.Name),
		"--count", "1").Output()
	if err != nil {
		return nil, err
	}
	var result struct{ Instances []struct{ InstanceId string; State struct{ Name string } } `json:"Instances"` }
	json.Unmarshal(out, &result)
	if len(result.Instances) == 0 {
		return nil, fmt.Errorf("instance not created")
	}
	return &provider.Server{ID: result.Instances[0].InstanceId, Name: opts.Name, Status: result.Instances[0].State.Name, Region: opts.Region, Size: opts.Size}, nil
}

func (p *Provider) DeleteServer(ctx context.Context, id string) error {
	return exec.CommandContext(ctx, "aws", "ec2", "terminate-instances", "--instance-ids", id).Run()
}

func (p *Provider) ListServers(ctx context.Context) ([]provider.Server, error) {
	out, err := exec.CommandContext(ctx, "aws", "ec2", "describe-instances",
		"--filters", "Name=instance-state-name,Values=running,stopped", "--output", "json").Output()
	if err != nil {
		return nil, err
	}
	var result struct{ Reservations []struct{ Instances []struct {
		InstanceId string; State struct{ Name string }; InstanceType string
		Placement struct{ AvailabilityZone string }
	} `json:"Instances"` } `json:"Reservations"` }
	json.Unmarshal(out, &result)
	var servers []provider.Server
	for _, r := range result.Reservations {
		for _, i := range r.Instances {
			servers = append(servers, provider.Server{ID: i.InstanceId, Status: i.State.Name, Size: i.InstanceType, Region: i.Placement.AvailabilityZone})
		}
	}
	return servers, nil
}

func (p *Provider) GetServer(ctx context.Context, id string) (*provider.Server, error) {
	out, err := exec.CommandContext(ctx, "aws", "ec2", "describe-instances", "--instance-ids", id, "--output", "json").Output()
	if err != nil {
		return nil, err
	}
	var result struct{ Reservations []struct{ Instances []struct{ InstanceId, InstanceType string; State struct{ Name string }; Placement struct{ AvailabilityZone string } } `json:"Instances"` } `json:"Reservations"` }
	json.Unmarshal(out, &result)
	for _, r := range result.Reservations {
		for _, i := range r.Instances {
			return &provider.Server{ID: i.InstanceId, Status: i.State.Name, Size: i.InstanceType, Region: i.Placement.AvailabilityZone}, nil
		}
	}
	return nil, fmt.Errorf("instance %s not found", id)
}

func (p *Provider) ListRegions(ctx context.Context) ([]provider.Region, error) {
	out, err := exec.CommandContext(ctx, "aws", "ec2", "describe-regions", "--output", "json").Output()
	if err != nil {
		return nil, err
	}
	var result struct{ Regions []struct{ RegionName, Endpoint string } `json:"Regions"` }
	json.Unmarshal(out, &result)
	regions := make([]provider.Region, 0, len(result.Regions))
	for _, r := range result.Regions {
		regions = append(regions, provider.Region{ID: r.RegionName, Name: r.RegionName})
	}
	return regions, nil
}

func (p *Provider) ListSizes(ctx context.Context) ([]provider.Size, error) {
	out, err := exec.CommandContext(ctx, "aws", "ec2", "describe-instance-types",
		"--filters", "Name=current-generation,Values=true", "--output", "json", "--max-items", "50").Output()
	if err != nil {
		return nil, err
	}
	var result struct{ InstanceTypes []struct {
		InstanceType string
		VCpuInfo     struct{ DefaultVCpus int } `json:"VCpuInfo"`
		MemoryInfo   struct{ SizeInMiB int }
	} `json:"InstanceTypes"` }
	json.Unmarshal(out, &result)
	sizes := make([]provider.Size, 0, len(result.InstanceTypes))
	for _, t := range result.InstanceTypes {
		sizes = append(sizes, provider.Size{ID: t.InstanceType, Name: t.InstanceType, VCPUs: t.VCpuInfo.DefaultVCpus, MemoryGB: float64(t.MemoryInfo.SizeInMiB) / 1024})
	}
	return sizes, nil
}

func (p *Provider) ListImages(ctx context.Context) ([]provider.Image, error) {
	out, err := exec.CommandContext(ctx, "aws", "ec2", "describe-images",
		"--owners", "amazon", "--filters", "Name=name,Values=ubuntu/images/hvm-ssd/ubuntu-*-24.04-amd64-server-*", "--output", "json").Output()
	if err != nil {
		return nil, err
	}
	var result struct{ Images []struct{ ImageId, Name string } `json:"Images"` }
	json.Unmarshal(out, &result)
	imgs := make([]provider.Image, 0, len(result.Images))
	for _, i := range result.Images {
		imgs = append(imgs, provider.Image{ID: i.ImageId, Name: i.Name})
	}
	return imgs, nil
}

func (p *Provider) CreateSSHKey(ctx context.Context, name string, key []byte) (*provider.SSHKey, error) {
	out, err := exec.CommandContext(ctx, "aws", "ec2", "import-key-pair", "--key-name", name, "--public-key-material", string(key), "--output", "json").Output()
	if err != nil {
		return nil, err
	}
	var k struct{ KeyName, KeyPairId string }
	json.Unmarshal(out, &k)
	return &provider.SSHKey{ID: k.KeyPairId, Name: k.KeyName}, nil
}

func (p *Provider) ResizeServer(ctx context.Context, id, size string) error {
	exec.CommandContext(ctx, "aws", "ec2", "stop-instances", "--instance-ids", id).Run()
	return exec.CommandContext(ctx, "aws", "ec2", "modify-instance-attribute", "--instance-id", id, "--instance-type", size).Run()
}
func (p *Provider) ShutdownServer(ctx context.Context, id string) error { return exec.CommandContext(ctx, "aws", "ec2", "stop-instances", "--instance-ids", id).Run() }
func (p *Provider) PowerOnServer(ctx context.Context, id string) error  { return exec.CommandContext(ctx, "aws", "ec2", "start-instances", "--instance-ids", id).Run() }

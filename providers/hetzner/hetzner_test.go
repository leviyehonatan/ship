package hetzner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/leviyehonatan/ship/internal/provider"
)

func TestParseServerList(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "servers.json"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	var servers []hcloudServer
	if err := json.Unmarshal(data, &servers); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
	}

	converted := make([]provider.Server, 0, len(servers))
	for _, s := range servers {
		converted = append(converted, provider.Server{
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

	// Server 1
	if converted[0].ID != "159919881" {
		t.Errorf("id = %q", converted[0].ID)
	}
	if converted[0].Name != "test-server" {
		t.Errorf("name = %q", converted[0].Name)
	}
	if converted[0].PublicIPv4 != "46.224.91.70" {
		t.Errorf("ipv4 = %q", converted[0].PublicIPv4)
	}
	if converted[0].Size != "cx23" {
		t.Errorf("size = %q", converted[0].Size)
	}
	if converted[0].Region != "nbg1" {
		t.Errorf("region = %q", converted[0].Region)
	}

	// Server 2 (offline)
	if converted[1].Status != "off" {
		t.Errorf("status = %q, want off", converted[1].Status)
	}
	if converted[1].Name != "staging" {
		t.Errorf("name = %q", converted[1].Name)
	}
}

func TestParseServerTypes(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "server_types.json"))
	if err != nil {
		t.Skipf("no fixture yet: %v", err)
	}

	var types []hcloudServerType
	if err := json.Unmarshal(data, &types); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	found := false
	for _, tp := range types {
		if tp.Name == "cx23" {
			found = true
			if tp.Cores != 2 {
				t.Errorf("cx23 cores = %d", tp.Cores)
			}
			if tp.Memory != 4.0 {
				t.Errorf("cx23 memory = %f", tp.Memory)
			}
			if tp.Disk != 40 {
				t.Errorf("cx23 disk = %d", tp.Disk)
			}
		}
	}
	if !found {
		t.Skip("cx23 not in fixture")
	}
}

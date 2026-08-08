package detect

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Status int

const (
	StatusNotFound  Status = iota
	StatusNotAuth   Status = iota
	StatusReady     Status = iota
	StatusWarning   Status = iota
)

func (s Status) String() string {
	switch s {
	case StatusNotFound:
		return "not installed"
	case StatusNotAuth:
		return "not authenticated"
	case StatusReady:
		return "ready"
	case StatusWarning:
		return "ready (version warning)"
	default:
		return "unknown"
	}
}

func (s Status) Icon() string {
	switch s {
	case StatusNotFound:
		return "-"
	case StatusNotAuth:
		return "~"
	case StatusReady:
		return "✓"
	case StatusWarning:
		return "⚠"
	default:
		return "?"
	}
}

type SystemProvider struct {
	Name            string
	CLI             string
	ConfigPaths     []string
	ValidateCmd     string
	VersionCmd      string
	MinVersion      string // semver, inclusive
	MaxTestedVersion string // semver, inclusive
}

type PlatformProvider struct {
	Name       string
	ConfigFile string
}

type ProviderInfo struct {
	Name    string
	Status  Status
	Version string
	Warning string
}

var SystemProviders = []SystemProvider{
	{
		Name:             "hetzner",
		CLI:              "hcloud",
		ConfigPaths:      []string{".config/hcloud/cli.toml"},
		ValidateCmd:      "hcloud server list",
		VersionCmd:       "hcloud version",
		MinVersion:       "1.0.0",
		MaxTestedVersion: "1.49.0",
	},
	{
		Name:             "linode",
		CLI:              "linode-cli",
		ConfigPaths:      []string{".config/linode-cli"},
		ValidateCmd:      "linode-cli linodes list --json",
		VersionCmd:       "linode-cli --version",
		MinVersion:       "5.0.0",
		MaxTestedVersion: "5.52.0",
	},
	{
		Name:             "digitalocean",
		CLI:              "doctl",
		ConfigPaths:      []string{".config/doctl/config.yaml"},
		ValidateCmd:      "doctl account get",
		VersionCmd:       "doctl version",
		MinVersion:       "1.0.0",
		MaxTestedVersion: "1.120.0",
	},
	{
		Name:             "vultr",
		CLI:              "vultr-cli",
		ConfigPaths:      []string{".vultr-cli.yaml"},
		ValidateCmd:      "vultr-cli instance list",
		VersionCmd:       "vultr-cli version",
		MinVersion:       "2.0.0",
		MaxTestedVersion: "3.2.0",
	},
}

var PlatformProviders = []PlatformProvider{
	{Name: "fly", ConfigFile: "fly.toml"},
	{Name: "vercel", ConfigFile: ".vercel/project.json"},
	{Name: "netlify", ConfigFile: ".netlify/state.json"},
	{Name: "supabase", ConfigFile: "supabase/config.toml"},
	{Name: "railway", ConfigFile: "railway.json"},
	{Name: "render", ConfigFile: "render.yaml"},
}

func DetectSystem(p SystemProvider) ProviderInfo {
	info := ProviderInfo{Name: p.Name}

	if _, err := exec.LookPath(p.CLI); err != nil {
		info.Status = StatusNotFound
		return info
	}

	home, err := os.UserHomeDir()
	if err != nil {
		info.Status = StatusNotFound
		return info
	}

	configFound := false
	for _, cp := range p.ConfigPaths {
		if _, err := os.Stat(filepath.Join(home, cp)); err == nil {
			configFound = true
			break
		}
	}
	if !configFound {
		info.Status = StatusNotFound
		return info
	}

	// Check auth
	parts := strings.Fields(p.ValidateCmd)
	if len(parts) == 0 {
		info.Status = StatusNotAuth
		return info
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		info.Status = StatusNotAuth
		return info
	}

	info.Status = StatusReady

	// Version check
	installedVer := getVersion(p.CLI, p.VersionCmd)
	info.Version = installedVer
	if installedVer != "" && p.MinVersion != "" && p.MaxTestedVersion != "" {
		warn := checkVersion(p.Name, installedVer, p.MinVersion, p.MaxTestedVersion)
		if warn != "" {
			info.Warning = warn
			info.Status = StatusWarning
		}
	}

	return info
}

func DetectPlatform(p PlatformProvider) bool {
	return DetectPlatformAt(p, "")
}

func DetectPlatformAt(p PlatformProvider, dir string) bool {
	if dir == "" {
		dir, _ = os.Getwd()
	}
	_, err := os.Stat(filepath.Join(dir, p.ConfigFile))
	return err == nil
}

// MustAuth ensures the provider CLI is installed and authenticated.
// Returns a user-friendly error message if not.
func MustAuth(p SystemProvider) error {
	if _, err := exec.LookPath(p.CLI); err != nil {
		return fmt.Errorf(
			"%s CLI not found.\n\n  Install: brew install %s\n  Then run: %s",
			p.Name, p.CLI, p.ValidateCmd,
		)
	}

	home, _ := os.UserHomeDir()
	configFound := false
	for _, cp := range p.ConfigPaths {
		if _, err := os.Stat(filepath.Join(home, cp)); err == nil {
			configFound = true
			break
		}
	}
	if !configFound {
		return fmt.Errorf(
			"%s is not configured.\n\n  Run: %s",
			p.Name, p.ValidateCmd,
		)
	}

	parts := strings.Fields(p.ValidateCmd)
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf(
			"%s is not authenticated.\n\n  Run: %s",
			p.Name, p.ValidateCmd,
		)
	}

	return nil
}

// ============================================================

var versionRe = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)

func getVersion(cli, versionCmd string) string {
	parts := strings.Fields(versionCmd)
	if len(parts) == 0 {
		return ""
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return ""
	}
	matches := versionRe.FindStringSubmatch(stdout.String())
	if len(matches) == 4 {
		return matches[0]
	}
	return ""
}

func checkVersion(name, installed, min, max string) string {
	inst := parseSemver(installed)
	minV := parseSemver(min)
	maxV := parseSemver(max)

	if compareSemver(inst, minV) < 0 {
		return fmt.Sprintf(
			"%s %s is too old (minimum %s required). Run: %s update",
			name, installed, min, name,
		)
	}
	if compareSemver(inst, maxV) > 0 {
		return fmt.Sprintf(
			"%s %s is newer than tested (%s). It should work but you may hit issues. Report them at github.com/leviyehonatan/ship/issues",
			name, installed, max,
		)
	}
	return ""
}

type semver [3]int

func parseSemver(v string) semver {
	m := versionRe.FindStringSubmatch(v)
	if len(m) != 4 {
		return semver{}
	}
	var sv semver
	sv[0], _ = strconv.Atoi(m[1])
	sv[1], _ = strconv.Atoi(m[2])
	sv[2], _ = strconv.Atoi(m[3])
	return sv
}

func compareSemver(a, b semver) int {
	for i := 0; i < 3; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

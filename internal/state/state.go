package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/leviyehonatan/ship/internal/config"
)

type Server struct {
	Name       string `json:"name"`
	ID         string `json:"id"`
	IP         string `json:"ip"`
	Provider   string `json:"provider"`
	Size       string `json:"size"`
	Region     string `json:"region"`
}

func SaveServer(s Server) error {
	dir := config.ServersDir()
	os.MkdirAll(dir, 0700)

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, s.Name+".json"), data, 0644)
}

func LoadServer(name string) (*Server, error) {
	data, err := os.ReadFile(filepath.Join(config.ServersDir(), name+".json"))
	if err != nil {
		return nil, err
	}
	var s Server
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func ListServers() ([]Server, error) {
	dir := config.ServersDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}
	var servers []Server
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		s, err := LoadServer(strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			continue
		}
		servers = append(servers, *s)
	}
	return servers, nil
}

func SetCurrent(name string) error {
	os.MkdirAll(config.StateDir(), 0700)
	return os.WriteFile(filepath.Join(config.StateDir(), "current-server"), []byte(name), 0644)
}

func Current() string {
	data, err := os.ReadFile(filepath.Join(config.StateDir(), "current-server"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

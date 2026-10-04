package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Store.Path != "data/lan2netbox.db" {
		t.Errorf("Store.Path = %q, want data/lan2netbox.db", cfg.Store.Path)
	}
	if cfg.Sniffer.ScanInterval != 60*time.Second {
		t.Errorf("Sniffer.ScanInterval = %v, want 60s", cfg.Sniffer.ScanInterval)
	}
	if cfg.Sniffer.OfflineTimeout != 10*time.Minute {
		t.Errorf("Sniffer.OfflineTimeout = %v, want 10m", cfg.Sniffer.OfflineTimeout)
	}
	if !cfg.Probe.Plugins.SNMP.Enabled || cfg.Probe.Plugins.SNMP.Community != "public" {
		t.Errorf("SNMP defaults не применены: %+v", cfg.Probe.Plugins.SNMP)
	}
	if !cfg.NetBox.AutoCreate {
		t.Error("NetBox.AutoCreate = false, want true")
	}
	if cfg.Web.Listen != "0.0.0.0:8080" {
		t.Errorf("Web.Listen = %q, want 0.0.0.0:8080", cfg.Web.Listen)
	}
	if cfg.Logging.Level != "info" || cfg.Logging.Format != "text" {
		t.Errorf("Logging defaults не применены: %+v", cfg.Logging)
	}
}

func TestLoad(t *testing.T) {
	yaml := `
store:
  path: /var/lib/lan2netbox/lan2netbox.db
sniffer:
  scan_interval: 30s
  subnets:
    - 10.0.0.0/8
    - 172.16.0.0/12
probe:
  plugins:
    snmp:
      community: secret
      version: 3
      username: user
      password: pass
netbox:
  enabled: false
`
	path := writeTemp(t, yaml)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	if cfg.Store.Path != "/var/lib/lan2netbox/lan2netbox.db" {
		t.Errorf("Store.Path = %q", cfg.Store.Path)
	}
	if cfg.Sniffer.ScanInterval != 30*time.Second {
		t.Errorf("ScanInterval = %v, want 30s", cfg.Sniffer.ScanInterval)
	}
	// Значения по умолчанию для отсутствующих ключей должны сохраняться.
	if cfg.Sniffer.OfflineTimeout != 10*time.Minute {
		t.Errorf("OfflineTimeout = %v, want default 10m", cfg.Sniffer.OfflineTimeout)
	}
	if len(cfg.Sniffer.Subnets) != 2 {
		t.Errorf("Subnets = %v, want 2 элемента", cfg.Sniffer.Subnets)
	}
	if cfg.Probe.Plugins.SNMP.Community != "secret" || cfg.Probe.Plugins.SNMP.Version != "3" {
		t.Errorf("SNMP = %+v", cfg.Probe.Plugins.SNMP)
	}
	if cfg.NetBox.Enabled {
		t.Error("NetBox.Enabled = true, want false")
	}
}

func TestLoadMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope.yaml")
	if _, err := Load(path); err == nil {
		t.Fatal("Load(): ожидалась ошибка для отсутствующего файла")
	}
}

func TestValidateErrors(t *testing.T) {
	yaml := `
sniffer:
  scan_interval: -5s
  capture: [llmnr]
netbox:
  enabled: true
  url: not-a-url
  token: ""
logging:
  level: verbose
`
	path := writeTemp(t, yaml)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load(): ожидалась ошибка валидации")
	}
	for _, want := range []string{
		"sniffer.scan_interval",
		"sniffer.capture",
		"netbox.url",
		"netbox.token",
		"logging.level",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("ошибка не содержит %q: %v", want, err)
		}
	}
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

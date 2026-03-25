package boot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadBootstrapConfig_UsesRegisteredFlagValueWithoutParsing(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "bootstrap.yaml")
	content := []byte("lynx:\n  application:\n    name: boot-test\n    version: v0.0.1\n")
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("failed to write bootstrap config: %v", err)
	}

	previousFlagConf := flagConf
	t.Cleanup(func() {
		flagConf = previousFlagConf
	})
	flagConf = configPath

	configMgr := GetConfigManager()
	configMgr.SetConfigPath("")

	app := &Application{}
	if err := app.LoadBootstrapConfig(); err != nil {
		t.Fatalf("expected bootstrap config load to succeed without flag.Parse(): %v", err)
	}
	t.Cleanup(func() {
		if app.cleanup != nil {
			app.cleanup()
		}
	})

	if got := configMgr.GetConfigPath(); got != configPath {
		t.Fatalf("expected config manager path to be populated from registered flag value, got %q", got)
	}
	if app.conf == nil {
		t.Fatal("expected application config to be initialized")
	}
}

func TestLoadBootstrapConfig_PrefersApplicationConfigPath(t *testing.T) {
	dir := t.TempDir()
	instanceConfigPath := filepath.Join(dir, "instance-bootstrap.yaml")
	managerConfigPath := filepath.Join(dir, "manager-bootstrap.yaml")

	if err := os.WriteFile(instanceConfigPath, []byte("lynx:\n  application:\n    name: instance\n    version: v1.0.0\n"), 0o600); err != nil {
		t.Fatalf("failed to write instance bootstrap config: %v", err)
	}
	if err := os.WriteFile(managerConfigPath, []byte("lynx:\n  application:\n    name: manager\n    version: v1.0.0\n"), 0o600); err != nil {
		t.Fatalf("failed to write manager bootstrap config: %v", err)
	}

	previousFlagConf := flagConf
	t.Cleanup(func() {
		flagConf = previousFlagConf
	})
	flagConf = ""

	configMgr := GetConfigManager()
	previousManagerPath := configMgr.GetConfigPath()
	t.Cleanup(func() {
		configMgr.SetConfigPath(previousManagerPath)
	})
	configMgr.SetConfigPath(managerConfigPath)

	app := (&Application{}).SetConfigPath(instanceConfigPath)
	if err := app.LoadBootstrapConfig(); err != nil {
		t.Fatalf("expected instance-scoped bootstrap config load to succeed: %v", err)
	}
	t.Cleanup(func() {
		if app.cleanup != nil {
			app.cleanup()
		}
	})

	if app.conf == nil {
		t.Fatal("expected application config to be initialized")
	}

	name, err := app.conf.Value("lynx.application.name").String()
	if err != nil {
		t.Fatalf("failed to read application name from config: %v", err)
	}
	if name != "instance" {
		t.Fatalf("expected instance-scoped config to win, got %q", name)
	}
	if got := configMgr.GetConfigPath(); got != managerConfigPath {
		t.Fatalf("expected global config manager path to remain unchanged, got %q", got)
	}
}

func TestResolveBootstrapConfigPath_PrefersInstancePath(t *testing.T) {
	previousFlagConf := flagConf
	t.Cleanup(func() {
		flagConf = previousFlagConf
	})
	flagConf = "/tmp/from-flag.yaml"

	configMgr := GetConfigManager()
	previousManagerPath := configMgr.GetConfigPath()
	t.Cleanup(func() {
		configMgr.SetConfigPath(previousManagerPath)
	})
	configMgr.SetConfigPath("/tmp/from-manager.yaml")

	app := (&Application{}).SetConfigPath("/tmp/from-instance.yaml")
	if got := app.resolveBootstrapConfigPath(); got != "/tmp/from-instance.yaml" {
		t.Fatalf("expected instance config path to win, got %q", got)
	}
}

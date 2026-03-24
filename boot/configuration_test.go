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

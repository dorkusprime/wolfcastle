package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitCmd_NewProject(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmp)

	oldApp := app
	defer func() { app = oldApp }()

	app = oldApp // use default app for init (no config needed)
	rootCmd.SetArgs([]string{"init"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// Check .wolfcastle directory was created
	wcDir := filepath.Join(tmp, ".wolfcastle")
	if _, err := os.Stat(wcDir); os.IsNotExist(err) {
		t.Error(".wolfcastle directory was not created")
	}

	// Check config files exist
	if _, err := os.Stat(filepath.Join(wcDir, "system", "base", "config.json")); os.IsNotExist(err) {
		t.Error("base/config.json was not created")
	}
}

func TestInitCmd_AlreadyInitialized(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmp)

	// Pre-create .wolfcastle
	_ = os.MkdirAll(filepath.Join(tmp, ".wolfcastle"), 0755)

	oldApp := app
	defer func() { app = oldApp }()

	rootCmd.SetArgs([]string{"init"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("init should succeed (exit 0) when already initialized: %v", err)
	}
}

func TestInitCmd_ForceReinit(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmp)

	// First init
	rootCmd.SetArgs([]string{"init"})
	_ = rootCmd.Execute()

	// Force reinit
	rootCmd.SetArgs([]string{"init", "--force"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("init --force failed: %v", err)
	}
}

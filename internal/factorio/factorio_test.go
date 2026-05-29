package factorio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureWorkDirsCreatesExpectedLayout(t *testing.T) {
	dirs, err := EnsureWorkDirs(filepath.Join(t.TempDir(), ".fmqa"), "run-1")
	if err != nil {
		t.Fatal(err)
	}

	for _, dir := range []string{dirs.Config, dirs.Saves, dirs.ScriptOutput, dirs.Mods, dirs.Run, dirs.Reports} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("expected directory %s: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("expected %s to be a directory", dir)
		}
	}
	if dirs.ConfigFile != filepath.Join(dirs.Config, "config.ini") {
		t.Fatalf("config file = %q, wanted it under %q", dirs.ConfigFile, dirs.Config)
	}
	if dirs.ServerSettings != filepath.Join(dirs.Config, "server-settings.json") {
		t.Fatalf("server settings = %q, wanted it under %q", dirs.ServerSettings, dirs.Config)
	}
}

func TestStageModsSymlinksUserModsAndControlMod(t *testing.T) {
	root := t.TempDir()
	userMods := filepath.Join(root, "user-mods")
	controlMod := filepath.Join(root, "qa_control_mod")
	stageDir := filepath.Join(root, "stage")
	if err := os.MkdirAll(filepath.Join(userMods, "debug-mod"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(controlMod, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := stageMods(stageDir, userMods, controlMod)
	if err != nil {
		t.Fatal(err)
	}
	if got != stageDir {
		t.Fatalf("stageMods returned %q, wanted %q", got, stageDir)
	}

	for _, name := range []string{"debug-mod", "qa_control_mod"} {
		link := filepath.Join(stageDir, name)
		info, err := os.Lstat(link)
		if err != nil {
			t.Fatalf("expected staged link %s: %v", link, err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("expected %s to be a symlink", link)
		}
	}
}

func TestWriteConfigUsesLocalWriteData(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config.ini")
	factorioBin := filepath.Join(root, "factorio", "bin", "x64", "factorio")
	writeData := filepath.Join(root, ".fmqa", "factorio")
	if err := writeConfig(configPath, factorioBin, writeData); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !containsLine(text, "write-data="+writeData) {
		t.Fatalf("config did not set write-data to local dir:\n%s", text)
	}
	if !containsLine(text, "read-data="+filepath.Join(root, "factorio", "data")) {
		t.Fatalf("config did not set read-data from binary path:\n%s", text)
	}
}

func TestWriteServerSettingsDisablesAutoPause(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server-settings.json")
	if err := writeServerSettings(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, `"auto_pause": false`) {
		t.Fatalf("server settings did not disable auto_pause:\n%s", text)
	}
	if !strings.Contains(text, `"public": false`) {
		t.Fatalf("server settings should not publish public games:\n%s", text)
	}
}

func TestScenarioNameMapsDefaultOpenWorld(t *testing.T) {
	if got := scenarioName("open_world"); got != "qa_control_mod/open_world" {
		t.Fatalf("scenarioName(open_world) = %q", got)
	}
	if got := scenarioName("base/freeplay"); got != "base/freeplay" {
		t.Fatalf("scenarioName(base/freeplay) = %q", got)
	}
}

func TestResolveFactorioBinAcceptsInstallDirectory(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin", "x64", "factorio")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveFactorioBin(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != bin {
		t.Fatalf("resolveFactorioBin() = %q, wanted %q", got, bin)
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := expandHome("~/factorio")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(home, "factorio") {
		t.Fatalf("expandHome() = %q", got)
	}
}

func containsLine(text string, line string) bool {
	for _, candidate := range strings.Split(text, "\n") {
		if candidate == line {
			return true
		}
	}
	return false
}

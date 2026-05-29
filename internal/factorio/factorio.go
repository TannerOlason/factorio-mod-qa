package factorio

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type WorkDirs struct {
	Root           string
	Factorio       string
	Config         string
	ConfigFile     string
	ServerSettings string
	Saves          string
	ScriptOutput   string
	Mods           string
	Run            string
	Reports        string
}

type StartOptions struct {
	FactorioBin    string
	WriteDir       string
	ModsPath       string
	ControlModPath string
	Scenario       string
	RunID          string
	RCONPort       int
	RCONPassword   string
	ExtraArgs      []string
}

type Process struct {
	Cmd     *exec.Cmd
	Dirs    WorkDirs
	RCON    string
	LogPath string
	cancel  context.CancelFunc
	done    chan error
}

func EnsureWorkDirs(root string, runID string) (WorkDirs, error) {
	if root == "" {
		root = ".fmqa"
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return WorkDirs{}, err
	}
	root = absRoot
	if runID == "" {
		runID = time.Now().UTC().Format("20060102T150405Z")
	}
	dirs := WorkDirs{
		Root:           root,
		Factorio:       filepath.Join(root, "factorio"),
		Config:         filepath.Join(root, "factorio", "config"),
		ConfigFile:     filepath.Join(root, "factorio", "config", "config.ini"),
		ServerSettings: filepath.Join(root, "factorio", "config", "server-settings.json"),
		Saves:          filepath.Join(root, "factorio", "saves"),
		ScriptOutput:   filepath.Join(root, "factorio", "script-output"),
		Mods:           filepath.Join(root, "factorio", "mods"),
		Run:            filepath.Join(root, "runs", runID),
		Reports:        filepath.Join(root, "reports"),
	}
	for _, dir := range []string{dirs.Config, dirs.Saves, dirs.ScriptOutput, dirs.Mods, dirs.Run, dirs.Reports} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return WorkDirs{}, err
		}
	}
	return dirs, nil
}

func AllocateTCPPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, errors.New("listener did not return a TCP address")
	}
	return addr.Port, nil
}

func stageMods(stageDir string, userModsPath string, controlModPath string) (string, error) {
	if userModsPath == "" && controlModPath == "" {
		return "", nil
	}
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		return "", err
	}
	existing, err := os.ReadDir(stageDir)
	if err != nil {
		return "", err
	}
	for _, entry := range existing {
		if err := os.Remove(filepath.Join(stageDir, entry.Name())); err != nil {
			return "", err
		}
	}

	if userModsPath != "" {
		entries, err := os.ReadDir(userModsPath)
		if err != nil {
			return "", err
		}
		for _, entry := range entries {
			target, err := filepath.Abs(filepath.Join(userModsPath, entry.Name()))
			if err != nil {
				return "", err
			}
			if err := os.Symlink(target, filepath.Join(stageDir, entry.Name())); err != nil {
				return "", err
			}
		}
	}

	if controlModPath != "" {
		target, err := filepath.Abs(controlModPath)
		if err != nil {
			return "", err
		}
		name := filepath.Base(target)
		linkPath := filepath.Join(stageDir, name)
		_ = os.Remove(linkPath)
		if err := os.Symlink(target, linkPath); err != nil {
			return "", err
		}
	}
	return stageDir, nil
}

func Start(ctx context.Context, opts StartOptions) (*Process, error) {
	if opts.FactorioBin == "" {
		return nil, errors.New("--factorio-bin is required")
	}
	factorioBin, err := ResolveFactorioBin(opts.FactorioBin)
	if err != nil {
		return nil, err
	}
	if opts.RCONPassword == "" {
		opts.RCONPassword = "fmqa-local"
	}
	if opts.RCONPort == 0 {
		port, err := AllocateTCPPort()
		if err != nil {
			return nil, err
		}
		opts.RCONPort = port
	}
	dirs, err := EnsureWorkDirs(opts.WriteDir, opts.RunID)
	if err != nil {
		return nil, err
	}
	if err := writeConfig(dirs.ConfigFile, factorioBin, dirs.Factorio); err != nil {
		return nil, err
	}
	if err := writeServerSettings(dirs.ServerSettings); err != nil {
		return nil, err
	}

	args := []string{
		"--config", dirs.ConfigFile,
		"--rcon-port", strconv.Itoa(opts.RCONPort),
		"--rcon-password", opts.RCONPassword,
		"--server-settings", dirs.ServerSettings,
	}
	modsDir, err := stageMods(dirs.Mods, opts.ModsPath, opts.ControlModPath)
	if err != nil {
		return nil, err
	}
	if modsDir != "" {
		args = append(args, "--mod-directory", modsDir)
	}
	if opts.Scenario != "" {
		args = append(args, "--start-server-load-scenario", scenarioName(opts.Scenario))
	} else {
		savePath := filepath.Join(dirs.Saves, "fmqa-autosave.zip")
		args = append(args, "--start-server", savePath)
	}
	args = append(args, opts.ExtraArgs...)

	runCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(runCtx, factorioBin, args...)
	cmd.Dir = dirs.Root

	logPath := filepath.Join(dirs.Run, "factorio.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		cancel()
		return nil, err
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		cancel()
		return nil, err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
		_ = logFile.Close()
	}()

	return &Process{
		Cmd:     cmd,
		Dirs:    dirs,
		RCON:    fmt.Sprintf("127.0.0.1:%d", opts.RCONPort),
		LogPath: logPath,
		cancel:  cancel,
		done:    done,
	}, nil
}

func writeServerSettings(path string) error {
	content := `{
  "name": "fmqa-local",
  "description": "Local Factorio Mod QA run",
  "visibility": {
    "public": false,
    "lan": false
  },
  "username": "",
  "token": "",
  "game_password": "",
  "require_user_verification": false,
  "max_players": 0,
  "ignore_player_limit_for_returning_players": false,
  "allow_commands": "admins-only",
  "autosave_interval": 0,
  "autosave_slots": 1,
  "afk_autokick_interval": 0,
  "auto_pause": false,
  "auto_pause_when_players_connect": false,
  "only_admins_can_pause_the_game": false
}
`
	return os.WriteFile(path, []byte(content), 0o644)
}

func (p *Process) Stop() error {
	if p == nil || p.Cmd == nil || p.Cmd.Process == nil {
		return nil
	}
	_ = p.Cmd.Process.Signal(syscall.SIGINT)
	select {
	case err := <-p.done:
		return err
	case <-time.After(5 * time.Second):
		if p.cancel != nil {
			p.cancel()
		}
		return p.Cmd.Process.Kill()
	}
}

func ResolveFactorioBin(value string) (string, error) {
	path, err := expandHome(value)
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		candidates := []string{
			filepath.Join(path, "bin", "x64", "factorio"),
			filepath.Join(path, "factorio"),
		}
		for _, candidate := range candidates {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, nil
			}
		}
		return "", fmt.Errorf("Factorio directory %s did not contain bin/x64/factorio", path)
	}
	if !filepath.IsAbs(path) {
		if found, err := exec.LookPath(path); err == nil {
			return found, nil
		}
		return filepath.Abs(path)
	}
	return path, nil
}

func expandHome(value string) (string, error) {
	if value == "~" {
		return os.UserHomeDir()
	}
	if strings.HasPrefix(value, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, value[2:]), nil
	}
	return value, nil
}

func writeConfig(path string, factorioBin string, writeData string) error {
	readData := filepath.Clean(filepath.Join(filepath.Dir(factorioBin), "..", "..", "data"))
	content := fmt.Sprintf(`[path]
read-data=%s
write-data=%s

[general]
locale=auto

[other]
enable-crash-log-uploading=false
check-updates=false
`, readData, writeData)
	return os.WriteFile(path, []byte(content), 0o644)
}

func scenarioName(name string) string {
	if name == "open_world" {
		return "qa_control_mod/open_world"
	}
	return name
}

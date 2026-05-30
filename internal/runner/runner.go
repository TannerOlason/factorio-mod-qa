package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"factorio-mod-qa/internal/blueprint"
	"factorio-mod-qa/internal/factorio"
	"factorio-mod-qa/internal/qa"
	"factorio-mod-qa/internal/rcon"
	"factorio-mod-qa/internal/reports"
	"factorio-mod-qa/internal/snapshot"
)

const (
	DefaultPassword = "fmqa-local"
	ExportCommand   = `/silent-command rcon.print(remote.call("qa_control_mod", "export_snapshot"))`
	SummaryCommand  = `/silent-command rcon.print(remote.call("qa_control_mod", "runtime_summary"))`
)

type RunOptions struct {
	FactorioBin      string
	WriteDir         string
	ModsPath         string
	ControlModPath   string
	Scenario         string
	RunID            string
	RCONPort         int
	RCONPassword     string
	Timeout          time.Duration
	Policy           snapshot.Policy
	QAScenario       string
	BlueprintPath    string
	BlueprintCopies  int
	BlueprintSpacing int
	BlueprintTicks   int
}

type DoctorOptions struct {
	FactorioBin    string
	ControlModPath string
}

func Doctor(opts DoctorOptions, out io.Writer) error {
	if opts.FactorioBin == "" {
		return errors.New("--factorio-bin is required")
	}
	factorioBin, err := factorio.ResolveFactorioBin(opts.FactorioBin)
	if err != nil {
		return err
	}
	version, err := factorioVersion(factorioBin)
	if err != nil {
		return err
	}
	controlMod := opts.ControlModPath
	if controlMod == "" {
		controlMod = "qa_control_mod"
	}
	infoPath := filepath.Join(controlMod, "info.json")
	if _, err := os.Stat(infoPath); err != nil {
		return fmt.Errorf("qa_control_mod info.json not found at %s: %w", infoPath, err)
	}

	fmt.Fprintf(out, "factorio_bin=%s\n", factorioBin)
	fmt.Fprintf(out, "factorio_version=%s\n", strings.TrimSpace(version))
	fmt.Fprintf(out, "qa_control_mod=%s\n", controlMod)
	fmt.Fprintln(out, "status=ok")
	return nil
}

func Run(ctx context.Context, opts RunOptions) error {
	if opts.Timeout == 0 {
		opts.Timeout = 2 * time.Minute
	}
	if opts.RCONPassword == "" {
		opts.RCONPassword = DefaultPassword
	}
	var blueprintDoc *blueprint.Document
	if opts.BlueprintPath != "" {
		var err error
		blueprintDoc, err = blueprint.DecodeInput(opts.BlueprintPath)
		if err != nil {
			return err
		}
	}
	proc, err := factorio.Start(ctx, factorio.StartOptions{
		FactorioBin:    opts.FactorioBin,
		WriteDir:       opts.WriteDir,
		ModsPath:       opts.ModsPath,
		ControlModPath: opts.ControlModPath,
		Scenario:       opts.Scenario,
		RunID:          opts.RunID,
		RCONPort:       opts.RCONPort,
		RCONPassword:   opts.RCONPassword,
	})
	if err != nil {
		return err
	}
	defer func() {
		if proc != nil {
			_ = proc.Stop()
		}
	}()

	client, err := WaitForRCON(ctx, proc.RCON, opts.RCONPassword, opts.Timeout)
	if err != nil {
		return fmt.Errorf("factorio started but RCON did not become ready; log: %s: %w", proc.LogPath, err)
	}
	initialLogPath := proc.LogPath
	defer func() {
		if client != nil {
			_ = client.Close()
		}
	}()

	raw, err := client.Command(ExportCommand)
	if err != nil {
		return err
	}
	snap, err := decodeSnapshotResponse(raw)
	if err != nil {
		return err
	}
	snapshotPath := filepath.Join(proc.Dirs.Run, "prototype_snapshot.json")
	if err := snapshot.Write(snapshotPath, snap); err != nil {
		return err
	}
	artifacts := map[string]string{
		"factorio_log":       initialLogPath,
		"prototype_snapshot": snapshotPath,
	}
	if blueprintDoc != nil {
		blueprintPath := filepath.Join(proc.Dirs.Run, "blueprint_input.json")
		data, err := json.MarshalIndent(blueprintDoc.Raw, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(blueprintPath, append(data, '\n'), 0o644); err != nil {
			return err
		}
		artifacts["blueprint_input"] = blueprintPath
	}

	analysis, err := snapshot.Analyze(snap, opts.Policy)
	if err != nil {
		return err
	}
	reportIssues := append([]snapshot.Issue{}, analysis.ReportableIssues...)

	session := &qa.Session{
		RCON:      client,
		Process:   proc,
		Snapshot:  snap,
		RunID:     opts.RunID,
		Artifacts: map[string]string{},
	}
	session.RestartFromSave = func(restartCtx context.Context, saveName string) error {
		if client != nil {
			_ = client.Close()
			client = nil
		}
		currentProc := proc
		if currentProc == nil {
			return errors.New("factorio process is not running")
		}
		if err := currentProc.Stop(); err != nil {
			return err
		}
		proc = nil

		port, err := rconPort(currentProc.RCON)
		if err != nil {
			return err
		}
		savePath := filepath.Join(currentProc.Dirs.Saves, saveName+".zip")
		restartProc, err := factorio.Start(restartCtx, factorio.StartOptions{
			FactorioBin:    opts.FactorioBin,
			WriteDir:       opts.WriteDir,
			ModsPath:       opts.ModsPath,
			ControlModPath: opts.ControlModPath,
			RunID:          opts.RunID,
			RCONPort:       port,
			RCONPassword:   opts.RCONPassword,
			SavePath:       savePath,
			LogName:        "factorio-reload-" + sanitizeRunID(saveName) + ".log",
		})
		if err != nil {
			return err
		}
		restartClient, err := WaitForRCON(restartCtx, restartProc.RCON, opts.RCONPassword, opts.Timeout)
		if err != nil {
			_ = restartProc.Stop()
			return fmt.Errorf("factorio restarted but RCON did not become ready; log: %s: %w", restartProc.LogPath, err)
		}
		proc = restartProc
		client = restartClient
		session.Process = restartProc
		session.RCON = restartClient
		session.Artifacts["factorio_log_reload_"+sanitizeArtifactKey(saveName)] = restartProc.LogPath
		return nil
	}
	scenarios, err := qa.SelectScenarios(opts.QAScenario, snap, blueprintDoc, qa.BlueprintOptions{
		Copies:     opts.BlueprintCopies,
		Spacing:    opts.BlueprintSpacing,
		TickWindow: opts.BlueprintTicks,
	})
	if err != nil {
		return err
	}
	scenarioRuns, err := qa.Runner{Scenarios: scenarios, TraceDir: proc.Dirs.Run}.Run(ctx, session)
	if err != nil {
		return err
	}
	for key, value := range session.Artifacts {
		artifacts[key] = value
	}
	scenarioSummaries := make([]reports.ScenarioSummary, 0, len(scenarioRuns))
	for _, scenarioRun := range scenarioRuns {
		reportIssues = append(reportIssues, scenarioRun.Issues...)
		artifactKey := "trace_" + sanitizeArtifactKey(scenarioRun.Name)
		artifacts[artifactKey] = scenarioRun.TracePath
		scenarioSummaries = append(scenarioSummaries, reports.ScenarioSummary{
			Name:       scenarioRun.Name,
			IssueCount: scenarioRun.IssueCount,
			TracePath:  scenarioRun.TracePath,
			Error:      scenarioRun.Error,
			DurationMS: scenarioRun.DurationMS,
		})
	}

	saveName := "fmqa-" + sanitizeRunID(opts.RunID)
	var saveResult struct {
		Save string `json:"save"`
	}
	if err := session.Dispatch("save", map[string]any{"name": saveName}, &saveResult); err != nil {
		return fmt.Errorf("snapshot, reports, and traces were prepared, but native save failed: %w", err)
	}
	if saveResult.Save == "" {
		saveResult.Save = saveName
	}
	artifacts["native_save"] = filepath.Join(proc.Dirs.Saves, saveResult.Save+".zip")

	summary := reports.Summary{
		RunID:        opts.RunID,
		SnapshotPath: snapshotPath,
		IssueCount:   len(reportIssues),
		Issues:       reportIssues,
		Artifacts:    artifacts,
		Scenarios:    scenarioSummaries,
	}
	if err := reports.Write(proc.Dirs.Reports, summary); err != nil {
		return err
	}
	return nil
}

func factorioVersion(factorioBin string) (string, error) {
	cmd := exec.Command(factorioBin, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to run %s --version: %w: %s", factorioBin, err, strings.TrimSpace(string(output)))
	}
	return firstLine(string(output)), nil
}

func firstLine(value string) string {
	value = strings.TrimSpace(value)
	if i := strings.IndexByte(value, '\n'); i >= 0 {
		return value[:i]
	}
	return value
}

func ExportSnapshot(address string, password string, out string, timeout time.Duration) error {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	client, err := rcon.Dial(address, password, timeout)
	if err != nil {
		return err
	}
	defer client.Close()
	raw, err := client.Command(ExportCommand)
	if err != nil {
		return err
	}
	snap, err := decodeSnapshotResponse(raw)
	if err != nil {
		return err
	}
	return snapshot.Write(out, snap)
}

func ValidateSnapshot(path string, reportDir string, policy snapshot.Policy) error {
	snap, err := snapshot.Load(path)
	if err != nil {
		return err
	}
	analysis, err := snapshot.Analyze(snap, policy)
	if err != nil {
		return err
	}
	if reportDir != "" {
		if err := reports.Write(reportDir, reports.Summary{
			SnapshotPath: path,
			IssueCount:   len(analysis.ReportableIssues),
			Issues:       analysis.ReportableIssues,
		}); err != nil {
			return err
		}
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(analysis.Issues)
}

func WaitForRCON(ctx context.Context, address string, password string, timeout time.Duration) (*rcon.Client, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		client, err := rcon.Dial(address, password, 3*time.Second)
		if err == nil {
			return client, nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	if lastErr == nil {
		lastErr = errors.New("timed out")
	}
	return nil, lastErr
}

func rconPort(address string) (int, error) {
	i := strings.LastIndex(address, ":")
	if i < 0 || i == len(address)-1 {
		return 0, fmt.Errorf("RCON address %q did not include a port", address)
	}
	port, err := strconv.Atoi(address[i+1:])
	if err != nil {
		return 0, fmt.Errorf("RCON address %q had invalid port: %w", address, err)
	}
	return port, nil
}

func decodeSnapshotResponse(raw string) (*snapshot.Snapshot, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("empty snapshot response")
	}
	jsonObject := extractJSONObject(raw)
	if jsonObject == "" {
		return nil, fmt.Errorf("snapshot response did not contain JSON object: %q", truncate(raw, 200))
	}
	return snapshot.Decode([]byte(jsonObject))
}

func extractJSONObject(raw string) string {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end < start {
		return ""
	}
	return raw[start : end+1]
}

func luaString(value string) string {
	return strconv.Quote(value)
}

func truncate(value string, n int) string {
	if len(value) <= n {
		return value
	}
	return value[:n] + "..."
}

func sanitizeRunID(value string) string {
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "run"
	}
	return b.String()
}

func sanitizeArtifactKey(value string) string {
	return strings.ReplaceAll(sanitizeRunID(value), "-", "_")
}

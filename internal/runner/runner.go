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

	"factorio-mod-qa/internal/factorio"
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
	FactorioBin    string
	WriteDir       string
	ModsPath       string
	ControlModPath string
	Scenario       string
	RunID          string
	RCONPort       int
	RCONPassword   string
	Timeout        time.Duration
	Policy         snapshot.Policy
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
	defer proc.Stop()

	client, err := WaitForRCON(ctx, proc.RCON, opts.RCONPassword, opts.Timeout)
	if err != nil {
		return fmt.Errorf("factorio started but RCON did not become ready; log: %s: %w", proc.LogPath, err)
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
	snapshotPath := filepath.Join(proc.Dirs.Run, "prototype_snapshot.json")
	if err := snapshot.Write(snapshotPath, snap); err != nil {
		return err
	}

	analysis, err := snapshot.Analyze(snap, opts.Policy)
	if err != nil {
		return err
	}
	reportIssues := append([]snapshot.Issue{}, analysis.ReportableIssues...)
	runtimeIssues, err := detectScriptEventGrowth(client, snap)
	if err != nil {
		runtimeIssue := snapshot.Issue{
			Code:     "runtime_probe_failed",
			Severity: "warning",
			Title:    "Runtime script-stress probe failed",
			Details:  map[string]any{"error": err.Error()},
		}
		reportIssues = append(reportIssues, runtimeIssue)
	} else {
		reportIssues = append(reportIssues, runtimeIssues...)
	}
	summary := reports.Summary{
		RunID:        opts.RunID,
		SnapshotPath: snapshotPath,
		IssueCount:   len(reportIssues),
		Issues:       reportIssues,
		Artifacts:    map[string]string{"factorio_log": proc.LogPath},
	}
	if err := reports.Write(proc.Dirs.Reports, summary); err != nil {
		return err
	}
	if _, err := client.Command(`/silent-command remote.call("qa_control_mod", "save", "fmqa-` + sanitizeRunID(opts.RunID) + `")`); err != nil {
		return fmt.Errorf("snapshot and reports were written, but native save failed: %w", err)
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

func detectScriptEventGrowth(client *rcon.Client, snap *snapshot.Snapshot) ([]snapshot.Issue, error) {
	if _, ok := snap.Entities["qa-ticking-machine"]; !ok {
		return nil, nil
	}
	if _, err := client.Command(`/silent-command remote.call("qa_control_mod", "reset_script_event_counts")`); err != nil {
		return nil, err
	}
	payload := map[string]any{
		"surface":  "nauvis",
		"entities": scriptStressEntities("qa-ticking-machine", 12),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	if _, err := client.Command(`/silent-command rcon.print(remote.call("qa_control_mod", "place_entities_batch", ` + luaString(string(data)) + `))`); err != nil {
		return nil, err
	}
	time.Sleep(5 * time.Second)
	raw, err := client.Command(`/silent-command rcon.print(remote.call("qa_control_mod", "script_event_counts"))`)
	if err != nil {
		return nil, err
	}
	counts := map[string]float64{}
	if err := json.Unmarshal([]byte(extractJSONObject(raw)), &counts); err != nil {
		return nil, err
	}
	var issues []snapshot.Issue
	for name, count := range counts {
		if count >= 100 {
			issues = append(issues, snapshot.Issue{
				Code:     "script_event_growth",
				Severity: "warning",
				Title:    fmt.Sprintf("Script event counter %s grew during stress probe", name),
				Details:  map[string]any{"counter": name, "count": count, "entity": "qa-ticking-machine"},
			})
		}
	}
	return issues, nil
}

func scriptStressEntities(name string, count int) []map[string]any {
	entities := make([]map[string]any, 0, count)
	for i := 0; i < count; i++ {
		entities = append(entities, map[string]any{
			"name": name,
			"position": map[string]float64{
				"x": float64(i % 4),
				"y": float64(i / 4),
			},
			"force": "player",
		})
	}
	return entities
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

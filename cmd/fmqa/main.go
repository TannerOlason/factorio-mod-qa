package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"factorio-mod-qa/internal/blueprint"
	"factorio-mod-qa/internal/modscan"
	"factorio-mod-qa/internal/runner"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "fmqa:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return flag.ErrHelp
	}
	switch args[0] {
	case "doctor":
		return doctorCommand(args[1:])
	case "run":
		return runCommand(args[1:])
	case "snapshot":
		return snapshotCommand(args[1:])
	case "validate":
		return validateCommand(args[1:])
	case "blueprint-test":
		return blueprintTestCommand(args[1:])
	case "inspect-mod":
		return inspectModCommand(args[1:])
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func blueprintTestCommand(args []string) error {
	fs := flag.NewFlagSet("blueprint-test", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("blueprint-test requires exactly one blueprint file path or exchange string")
	}
	doc, err := blueprint.DecodeInput(fs.Arg(0))
	if err != nil {
		return err
	}
	return writeJSON(os.Stdout, doc)
}

func inspectModCommand(args []string) error {
	fs := flag.NewFlagSet("inspect-mod", flag.ContinueOnError)
	modDir := fs.String("mod-dir", "", "path to an unpacked Factorio mod directory")
	megaBaseEntities := fs.Int("mega-base-entities", 10000, "assumed entity count per entity-backed storage table for tick-loop estimates")
	if err := fs.Parse(args); err != nil {
		return err
	}
	report, err := modscan.Scan(modscan.Options{
		ModDir:           *modDir,
		MegaBaseEntities: *megaBaseEntities,
	})
	if err != nil {
		return err
	}
	return writeJSON(os.Stdout, report)
}

func doctorCommand(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	factorioBin := fs.String("factorio-bin", "", "path to the local Factorio binary or install directory")
	controlMod := fs.String("qa-control-mod", defaultControlModPath(), "path to qa_control_mod")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return runner.Doctor(runner.DoctorOptions{
		FactorioBin:    *factorioBin,
		ControlModPath: *controlMod,
	}, os.Stdout)
}

func runCommand(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to a JSON runner config")
	factorioBin := fs.String("factorio-bin", "", "path to the local Factorio binary or install directory")
	writeDir := fs.String("write-dir", ".fmqa", "fmqa working directory")
	modsPath := fs.String("mods-path", "", "path to a Factorio mods directory")
	controlMod := fs.String("qa-control-mod", defaultControlModPath(), "path to qa_control_mod")
	scenario := fs.String("scenario", "open_world", "scenario to start")
	qaScenario := fs.String("qa-scenario", "all", "QA scenario to run: all, script-event-growth, inventory-abuse, save-load-abuse, surface-spawn, or blueprint-smoke")
	blueprintPath := fs.String("blueprint", "", "blueprint file path or exchange string for blueprint-smoke")
	blueprintCopies := fs.Int("blueprint-copies", 1, "number of instant blueprint copies to place for blueprint-smoke")
	blueprintSpacing := fs.Int("blueprint-spacing", 12, "tile spacing between instant blueprint copies")
	blueprintTicks := fs.Int("blueprint-ticks", 120, "game ticks to wait after blueprint placement")
	runID := fs.String("run-id", time.Now().UTC().Format("20060102T150405Z"), "run identifier")
	rconPort := fs.Int("rcon-port", 0, "RCON port; 0 picks a free local port")
	rconPassword := fs.String("rcon-password", runner.DefaultPassword, "RCON password")
	timeout := fs.Duration("timeout", 2*time.Minute, "startup timeout")
	minReportSeverity := fs.String("min-report-severity", "", "minimum severity to include in reports: info, warning, or error")
	var positiveLoopWhitelist stringSliceFlag
	var suppressIssueCodes stringSliceFlag
	fs.Var(&positiveLoopWhitelist, "positive-loop-whitelist", "recipe name to ignore for positive same-material output loop checks")
	fs.Var(&suppressIssueCodes, "suppress-issue-code", "static validator issue code to suppress")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := runner.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	seen := visitedFlags(fs)
	applyRunConfig(cfg, seen, factorioBin, writeDir, modsPath, controlMod, scenario, qaScenario, blueprintPath, blueprintCopies, blueprintSpacing, blueprintTicks, runID, rconPort, rconPassword, timeout)
	policy := cfg.Policy()
	for _, recipe := range positiveLoopWhitelist {
		policy.PositiveLoopWhitelist[recipe] = true
	}
	for _, code := range suppressIssueCodes {
		policy.SuppressIssueCodes[code] = true
	}
	if *minReportSeverity != "" {
		policy.MinReportSeverity = *minReportSeverity
	}
	return runner.Run(context.Background(), runner.RunOptions{
		FactorioBin:      *factorioBin,
		WriteDir:         *writeDir,
		ModsPath:         *modsPath,
		ControlModPath:   *controlMod,
		Scenario:         *scenario,
		RunID:            *runID,
		RCONPort:         *rconPort,
		RCONPassword:     *rconPassword,
		Timeout:          *timeout,
		Policy:           policy,
		QAScenario:       *qaScenario,
		BlueprintPath:    *blueprintPath,
		BlueprintCopies:  *blueprintCopies,
		BlueprintSpacing: *blueprintSpacing,
		BlueprintTicks:   *blueprintTicks,
	})
}

func snapshotCommand(args []string) error {
	fs := flag.NewFlagSet("snapshot", flag.ContinueOnError)
	address := fs.String("rcon", "localhost:27000", "RCON host:port")
	password := fs.String("rcon-password", runner.DefaultPassword, "RCON password")
	out := fs.String("out", "prototype_snapshot.json", "snapshot output path")
	timeout := fs.Duration("timeout", 30*time.Second, "RCON timeout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return runner.ExportSnapshot(*address, *password, *out, *timeout)
}

func validateCommand(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to a JSON runner config")
	snapshot := fs.String("snapshot", "", "prototype snapshot JSON path")
	reportDir := fs.String("reports-dir", "", "optional directory for summary reports")
	minReportSeverity := fs.String("min-report-severity", "", "minimum severity to include in reports: info, warning, or error")
	var positiveLoopWhitelist stringSliceFlag
	var suppressIssueCodes stringSliceFlag
	fs.Var(&positiveLoopWhitelist, "positive-loop-whitelist", "recipe name to ignore for positive same-material output loop checks")
	fs.Var(&suppressIssueCodes, "suppress-issue-code", "static validator issue code to suppress")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := runner.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	seen := visitedFlags(fs)
	if !seen["snapshot"] && cfg.Snapshot != "" {
		*snapshot = cfg.Snapshot
	}
	if !seen["reports-dir"] && cfg.ReportsDir != "" {
		*reportDir = cfg.ReportsDir
	}
	if *snapshot == "" {
		return fmt.Errorf("--snapshot is required")
	}
	policy := cfg.Policy()
	for _, recipe := range positiveLoopWhitelist {
		policy.PositiveLoopWhitelist[recipe] = true
	}
	for _, code := range suppressIssueCodes {
		policy.SuppressIssueCodes[code] = true
	}
	if *minReportSeverity != "" {
		policy.MinReportSeverity = *minReportSeverity
	}
	return runner.ValidateSnapshot(*snapshot, *reportDir, policy)
}

func writeJSON(out *os.File, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(data))
	return err
}

func usage() {
	fmt.Fprintln(os.Stderr, `Usage:
  fmqa run --factorio-bin /path/to/factorio --write-dir .fmqa --mods-path /path/to/mods --scenario open_world --qa-scenario all --run-id local-test
  fmqa run --qa-scenario blueprint-smoke --blueprint ./blueprints/foo.txt --blueprint-copies 25 --blueprint-ticks 300
  fmqa doctor --factorio-bin /path/to/factorio
  fmqa snapshot --rcon localhost:27000 --out prototype_snapshot.json
  fmqa validate --snapshot prototype_snapshot.json
  fmqa validate --config qa_config.json
  fmqa blueprint-test ./blueprints/foo.txt
  fmqa inspect-mod --mod-dir /path/to/unpacked-mod --mega-base-entities 10000`)
}

func defaultControlModPath() string {
	if _, err := os.Stat("qa_control_mod"); err == nil {
		return "qa_control_mod"
	}
	exe, err := os.Executable()
	if err != nil {
		return "qa_control_mod"
	}
	return filepath.Join(filepath.Dir(exe), "..", "..", "qa_control_mod")
}

type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return fmt.Sprint([]string(*s))
}

func (s *stringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func visitedFlags(fs *flag.FlagSet) map[string]bool {
	seen := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		seen[f.Name] = true
	})
	return seen
}

func applyRunConfig(
	cfg runner.Config,
	seen map[string]bool,
	factorioBin *string,
	writeDir *string,
	modsPath *string,
	controlMod *string,
	scenario *string,
	qaScenario *string,
	blueprintPath *string,
	blueprintCopies *int,
	blueprintSpacing *int,
	blueprintTicks *int,
	runID *string,
	rconPort *int,
	rconPassword *string,
	timeout *time.Duration,
) {
	if !seen["factorio-bin"] && cfg.FactorioBin != "" {
		*factorioBin = cfg.FactorioBin
	}
	if !seen["write-dir"] && cfg.WriteDir != "" {
		*writeDir = cfg.WriteDir
	}
	if !seen["mods-path"] && cfg.ModsPath != "" {
		*modsPath = cfg.ModsPath
	}
	if !seen["qa-control-mod"] && cfg.ControlModPath != "" {
		*controlMod = cfg.ControlModPath
	}
	if !seen["scenario"] && cfg.Scenario != "" {
		*scenario = cfg.Scenario
	}
	if !seen["qa-scenario"] && cfg.QAScenario != "" {
		*qaScenario = cfg.QAScenario
	}
	if !seen["blueprint"] && cfg.BlueprintPath != "" {
		*blueprintPath = cfg.BlueprintPath
	}
	if !seen["blueprint-copies"] && cfg.BlueprintCopies != 0 {
		*blueprintCopies = cfg.BlueprintCopies
	}
	if !seen["blueprint-spacing"] && cfg.BlueprintSpacing != 0 {
		*blueprintSpacing = cfg.BlueprintSpacing
	}
	if !seen["blueprint-ticks"] && cfg.BlueprintTicks != 0 {
		*blueprintTicks = cfg.BlueprintTicks
	}
	if !seen["run-id"] && cfg.RunID != "" {
		*runID = cfg.RunID
	}
	if !seen["rcon-port"] && cfg.RCONPort != 0 {
		*rconPort = cfg.RCONPort
	}
	if !seen["rcon-password"] && cfg.RCONPassword != "" {
		*rconPassword = cfg.RCONPassword
	}
	if !seen["timeout"] && cfg.Timeout != 0 {
		*timeout = cfg.Timeout
	}
}

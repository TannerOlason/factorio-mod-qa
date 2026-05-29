package runner

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestLoadConfigLoadsStaticPolicy(t *testing.T) {
	path := writeTestConfig(t, t.TempDir(), "qa_config.json", `{
  "factorio_bin": "/opt/factorio",
  "write_dir": ".fmqa/custom",
  "mods_path": "fixtures/mods",
  "qa_control_mod": "qa_control_mod",
  "scenario": "open_world",
  "run_id": "config-run",
  "rcon_port": 27042,
  "rcon_password": "secret",
  "timeout": "45s",
  "snapshot": "prototype_snapshot.json",
  "reports_dir": ".fmqa/reports",
  "qa_scenario": "script-event-growth",
  "positive_loop_whitelist": ["known-loop"],
  "suppress_issue_codes": ["item_without_use"],
  "suppress_issue_matches": [
    {
      "code": "recipe_missing_crafting_machine",
      "details": {"recipe": "debug-plate"}
    }
  ],
  "severity_overrides": {"positive_output_loop": "info"},
  "severity_override_matches": [
    {
      "code": "fluid_source_sink_gap",
      "details": {"fluid": "debug-fluid"},
      "severity": "info"
    }
  ],
  "min_report_severity": "warning"
}`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.FactorioBin != "/opt/factorio" ||
		cfg.WriteDir != ".fmqa/custom" ||
		cfg.ModsPath != "fixtures/mods" ||
		cfg.ControlModPath != "qa_control_mod" ||
		cfg.RunID != "config-run" ||
		cfg.RCONPort != 27042 ||
		cfg.RCONPassword != "secret" ||
		cfg.Timeout != 45*time.Second ||
		cfg.Snapshot != "prototype_snapshot.json" ||
		cfg.ReportsDir != ".fmqa/reports" ||
		cfg.QAScenario != "script-event-growth" {
		t.Fatalf("config did not load expected scalar values: %#v", cfg)
	}
	if !reflect.DeepEqual(cfg.PositiveLoopWhitelist, []string{"known-loop"}) {
		t.Fatalf("positive loop whitelist = %#v", cfg.PositiveLoopWhitelist)
	}
	if !reflect.DeepEqual(cfg.SuppressIssueCodes, []string{"item_without_use"}) {
		t.Fatalf("suppress issue codes = %#v", cfg.SuppressIssueCodes)
	}
	if cfg.SuppressIssueMatches[0].Details["recipe"] != "debug-plate" {
		t.Fatalf("suppress match did not decode details: %#v", cfg.SuppressIssueMatches)
	}
	if cfg.SeverityOverrides["positive_output_loop"] != "info" {
		t.Fatalf("severity overrides = %#v", cfg.SeverityOverrides)
	}
	if cfg.SeverityOverrideMatches[0].Severity != "info" {
		t.Fatalf("severity override matches = %#v", cfg.SeverityOverrideMatches)
	}
	if cfg.MinReportSeverity != "warning" {
		t.Fatalf("min report severity = %q", cfg.MinReportSeverity)
	}
}

func TestLoadConfigExtendsAndAppliesPolicyProfiles(t *testing.T) {
	root := t.TempDir()
	writeTestConfig(t, root, "shared_policy.json", `{
  "enabled_static_policy_profiles": ["quiet-base"],
  "static_policy_profiles": {
    "quiet-base": {
      "suppress_issue_codes": "item_without_use",
      "severity_overrides": {"fluid_source_sink_gap": "info"}
    }
  },
  "positive_loop_whitelist": "kovarex-enrichment-process"
}`)
	path := writeTestConfig(t, root, "modpack.json", `{
  "extends": "shared_policy.json",
  "suppress_issue_codes": ["positive_output_loop"],
  "severity_overrides": {"item_without_use": "warning"}
}`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.PositiveLoopWhitelist, []string{"kovarex-enrichment-process"}) {
		t.Fatalf("positive loop whitelist = %#v", cfg.PositiveLoopWhitelist)
	}
	if !reflect.DeepEqual(cfg.SuppressIssueCodes, []string{"item_without_use", "positive_output_loop"}) {
		t.Fatalf("suppress issue codes = %#v", cfg.SuppressIssueCodes)
	}
	wantSeverityOverrides := map[string]string{
		"fluid_source_sink_gap": "info",
		"item_without_use":      "warning",
	}
	if !reflect.DeepEqual(cfg.SeverityOverrides, wantSeverityOverrides) {
		t.Fatalf("severity overrides = %#v, want %#v", cfg.SeverityOverrides, wantSeverityOverrides)
	}
}

func TestLoadConfigRejectsInvalidFields(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{
		"unknown.json":      `{"surprise": true}`,
		"bad_list.json":     `{"suppress_issue_matches": "recipe_missing_crafting_machine"}`,
		"bad_dict.json":     `{"severity_overrides": ["positive_output_loop"]}`,
		"bad_severity.json": `{"min_report_severity": "very-serious"}`,
		"missing_profile.json": `{
  "enabled_static_policy_profiles": ["missing"]
}`,
	} {
		path := writeTestConfig(t, root, name, content)
		if _, err := LoadConfig(path); err == nil {
			t.Fatalf("expected error for %s", name)
		}
	}
}

func writeTestConfig(t *testing.T, root string, name string, content string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

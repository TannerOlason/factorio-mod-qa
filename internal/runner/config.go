package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"factorio-mod-qa/internal/snapshot"
)

type Config struct {
	FactorioBin    string
	WriteDir       string
	ModsPath       string
	ControlModPath string
	Scenario       string
	RunID          string
	RCONPort       int
	RCONPassword   string
	Timeout        time.Duration
	Snapshot       string
	ReportsDir     string
	QAScenario     string

	PositiveLoopWhitelist      []string
	SuppressIssueCodes         []string
	SuppressIssueMatches       []snapshot.IssueMatch
	SeverityOverrides          map[string]string
	SeverityOverrideMatches    []snapshot.IssueMatch
	MinReportSeverity          string
	EnabledStaticPolicyProfile []string
}

var configKeys = map[string]bool{
	"extends":                        true,
	"factorio_bin":                   true,
	"write_dir":                      true,
	"mods_path":                      true,
	"qa_control_mod":                 true,
	"scenario":                       true,
	"run_id":                         true,
	"rcon_port":                      true,
	"rcon_password":                  true,
	"timeout":                        true,
	"snapshot":                       true,
	"reports_dir":                    true,
	"qa_scenario":                    true,
	"positive_loop_whitelist":        true,
	"suppress_issue_codes":           true,
	"suppress_issue_matches":         true,
	"severity_overrides":             true,
	"severity_override_matches":      true,
	"min_report_severity":            true,
	"static_policy_profiles":         true,
	"enabled_static_policy_profiles": true,
	"address":                        true,
	"goals":                          true,
	"max_traces":                     true,
	"mutations":                      true,
	"output_dir":                     true,
	"seed":                           true,
	"llm_planning":                   true,
	"native_saves":                   true,
	"validate_only":                  true,
	"start_cluster":                  true,
}

var listConfigKeys = map[string]bool{
	"positive_loop_whitelist":        true,
	"suppress_issue_codes":           true,
	"suppress_issue_matches":         true,
	"severity_override_matches":      true,
	"enabled_static_policy_profiles": true,
}

var scalarListConfigKeys = map[string]bool{
	"positive_loop_whitelist":        true,
	"suppress_issue_codes":           true,
	"enabled_static_policy_profiles": true,
}

var dictConfigKeys = map[string]bool{
	"severity_overrides":     true,
	"static_policy_profiles": true,
}

var staticPolicyProfileKeys = map[string]bool{
	"positive_loop_whitelist":   true,
	"suppress_issue_codes":      true,
	"suppress_issue_matches":    true,
	"severity_overrides":        true,
	"severity_override_matches": true,
	"min_report_severity":       true,
}

func LoadConfig(path string) (Config, error) {
	if path == "" {
		return Config{}, nil
	}
	data, err := loadConfigData(path, map[string]bool{})
	if err != nil {
		return Config{}, err
	}
	data, err = applyStaticPolicyProfiles(data)
	if err != nil {
		return Config{}, err
	}
	return configFromData(data)
}

func (c Config) Policy() snapshot.Policy {
	policy := snapshot.Policy{
		PositiveLoopWhitelist: stringSet(c.PositiveLoopWhitelist),
		SuppressIssueCodes:    stringSet(c.SuppressIssueCodes),
		SuppressIssueMatches:  c.SuppressIssueMatches,
		SeverityOverrides:     c.SeverityOverrides,
		SeverityOverrideRules: c.SeverityOverrideMatches,
		MinReportSeverity:     c.MinReportSeverity,
	}
	if policy.SeverityOverrides == nil {
		policy.SeverityOverrides = map[string]string{}
	}
	return policy
}

func loadConfigData(path string, seen map[string]bool) (map[string]any, error) {
	configPath, err := filepath.Abs(expandConfigPath(path))
	if err != nil {
		return nil, err
	}
	if seen[configPath] {
		return nil, fmt.Errorf("config extends cycle detected at %s", configPath)
	}
	seen[configPath] = true
	defer delete(seen, configPath)

	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, err
	}
	for key := range data {
		if !configKeys[key] {
			return nil, fmt.Errorf("unknown runner config option(s): %s", key)
		}
	}

	extends, err := configStringList(data["extends"], "extends", true)
	if err != nil {
		return nil, err
	}
	inherited := map[string]any{}
	for _, parent := range extends {
		parentPath := expandConfigPath(parent)
		if !filepath.IsAbs(parentPath) {
			parentPath = filepath.Join(filepath.Dir(configPath), parentPath)
		}
		parentData, err := loadConfigData(parentPath, seen)
		if err != nil {
			return nil, err
		}
		inherited, err = mergeConfigData(inherited, parentData)
		if err != nil {
			return nil, err
		}
	}
	return mergeConfigData(inherited, data)
}

func mergeConfigData(base map[string]any, override map[string]any) (map[string]any, error) {
	merged := map[string]any{}
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range override {
		if key == "extends" {
			continue
		}
		switch {
		case listConfigKeys[key]:
			existing, err := configList(merged[key], key)
			if err != nil {
				return nil, err
			}
			added, err := configList(value, key)
			if err != nil {
				return nil, err
			}
			merged[key] = append(existing, added...)
		case dictConfigKeys[key]:
			existing, err := configMap(merged[key], key)
			if err != nil {
				return nil, err
			}
			added, err := configMap(value, key)
			if err != nil {
				return nil, err
			}
			for addedKey, addedValue := range added {
				existing[addedKey] = addedValue
			}
			merged[key] = existing
		default:
			merged[key] = value
		}
	}
	return merged, nil
}

func applyStaticPolicyProfiles(data map[string]any) (map[string]any, error) {
	enabled, err := configStringList(data["enabled_static_policy_profiles"], "enabled_static_policy_profiles", true)
	if err != nil {
		return nil, err
	}
	profiles, err := configMap(data["static_policy_profiles"], "static_policy_profiles")
	if err != nil {
		return nil, err
	}

	profileOptions := map[string]any{}
	for _, name := range enabled {
		rawProfile, ok := profiles[name]
		if !ok {
			return nil, fmt.Errorf("unknown static policy profile: %s", name)
		}
		profile, ok := rawProfile.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("static policy profile %s must be a JSON object", name)
		}
		for key := range profile {
			if !staticPolicyProfileKeys[key] {
				return nil, fmt.Errorf("unknown static policy profile option in %s: %s", name, key)
			}
		}
		var err error
		profileOptions, err = mergeConfigData(profileOptions, profile)
		if err != nil {
			return nil, err
		}
	}
	return mergeConfigData(profileOptions, data)
}

func configFromData(data map[string]any) (Config, error) {
	cfg := Config{
		SeverityOverrides: map[string]string{},
	}
	var err error
	cfg.FactorioBin = stringValue(data["factorio_bin"])
	cfg.WriteDir = stringValue(data["write_dir"])
	cfg.ModsPath = stringValue(data["mods_path"])
	cfg.ControlModPath = stringValue(data["qa_control_mod"])
	cfg.Scenario = stringValue(data["scenario"])
	cfg.RunID = stringValue(data["run_id"])
	cfg.RCONPassword = stringValue(data["rcon_password"])
	cfg.Snapshot = stringValue(data["snapshot"])
	cfg.ReportsDir = stringValue(data["reports_dir"])
	cfg.QAScenario = stringValue(data["qa_scenario"])
	cfg.MinReportSeverity = stringValue(data["min_report_severity"])
	if cfg.MinReportSeverity != "" {
		if _, ok := severityNameSet()[cfg.MinReportSeverity]; !ok {
			return Config{}, fmt.Errorf("min_report_severity must be one of: info, warning, error")
		}
	}
	cfg.RCONPort, err = intValue(data["rcon_port"], "rcon_port")
	if err != nil {
		return Config{}, err
	}
	cfg.Timeout, err = durationValue(data["timeout"], "timeout")
	if err != nil {
		return Config{}, err
	}
	cfg.PositiveLoopWhitelist, err = configStringList(data["positive_loop_whitelist"], "positive_loop_whitelist", true)
	if err != nil {
		return Config{}, err
	}
	cfg.SuppressIssueCodes, err = configStringList(data["suppress_issue_codes"], "suppress_issue_codes", true)
	if err != nil {
		return Config{}, err
	}
	cfg.EnabledStaticPolicyProfile, err = configStringList(data["enabled_static_policy_profiles"], "enabled_static_policy_profiles", true)
	if err != nil {
		return Config{}, err
	}
	cfg.SuppressIssueMatches, err = matchList(data["suppress_issue_matches"], "suppress_issue_matches")
	if err != nil {
		return Config{}, err
	}
	cfg.SeverityOverrideMatches, err = matchList(data["severity_override_matches"], "severity_override_matches")
	if err != nil {
		return Config{}, err
	}
	overrides, err := configMap(data["severity_overrides"], "severity_overrides")
	if err != nil {
		return Config{}, err
	}
	for key, value := range overrides {
		text, ok := value.(string)
		if !ok {
			return Config{}, fmt.Errorf("severity_overrides values must be strings")
		}
		cfg.SeverityOverrides[key] = text
	}
	if err := cfg.Policy().Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func expandConfigPath(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if len(path) > 2 && path[:2] == "~/" {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func configList(value any, key string) ([]any, error) {
	if value == nil {
		return nil, nil
	}
	if text, ok := value.(string); ok && scalarListConfigKeys[key] {
		return []any{text}, nil
	}
	list, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("runner config option %s must be a JSON array", key)
	}
	return append([]any{}, list...), nil
}

func configStringList(value any, key string, allowScalar bool) ([]string, error) {
	if value == nil {
		return nil, nil
	}
	if text, ok := value.(string); ok && allowScalar {
		return []string{text}, nil
	}
	list, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("runner config option %s must be a JSON array", key)
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		text, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("runner config option %s must contain strings", key)
		}
		out = append(out, text)
	}
	return out, nil
}

func configMap(value any, key string) (map[string]any, error) {
	if value == nil {
		return map[string]any{}, nil
	}
	out, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("runner config option %s must be a JSON object", key)
	}
	return out, nil
}

func matchList(value any, key string) ([]snapshot.IssueMatch, error) {
	if value == nil {
		return nil, nil
	}
	list, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("runner config option %s must be a JSON array", key)
	}
	data, err := json.Marshal(list)
	if err != nil {
		return nil, err
	}
	var rules []snapshot.IssueMatch
	if err := json.Unmarshal(data, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func intValue(value any, key string) (int, error) {
	if value == nil {
		return 0, nil
	}
	number, ok := value.(float64)
	if !ok {
		return 0, fmt.Errorf("runner config option %s must be a number", key)
	}
	return int(number), nil
}

func durationValue(value any, key string) (time.Duration, error) {
	if value == nil {
		return 0, nil
	}
	switch v := value.(type) {
	case string:
		duration, err := time.ParseDuration(v)
		if err != nil {
			return 0, fmt.Errorf("runner config option %s must be a duration: %w", key, err)
		}
		return duration, nil
	case float64:
		return time.Duration(v) * time.Second, nil
	default:
		return 0, fmt.Errorf("runner config option %s must be a duration string or seconds", key)
	}
}

func stringSet(values []string) map[string]bool {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	return set
}

func severityNameSet() map[string]bool {
	return map[string]bool{"info": true, "warning": true, "error": true}
}

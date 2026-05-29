package snapshot

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

var severityRanks = map[string]int{
	"info":    0,
	"warning": 1,
	"error":   2,
}

var suppressionSeverities = map[string]bool{
	"ignore":     true,
	"off":        true,
	"suppress":   true,
	"suppressed": true,
}

type Policy struct {
	PositiveLoopWhitelist map[string]bool
	SuppressIssueCodes    map[string]bool
	SuppressIssueMatches  []IssueMatch
	SeverityOverrides     map[string]string
	SeverityOverrideRules []IssueMatch
	MinReportSeverity     string
}

type IssueMatch struct {
	Code            string            `json:"code,omitempty"`
	Prototype       string            `json:"prototype,omitempty"`
	Mod             string            `json:"mod,omitempty"`
	Details         map[string]any    `json:"details,omitempty"`
	DetailsContains map[string]any    `json:"details_contains,omitempty"`
	DetailsRegex    map[string]string `json:"details_regex,omitempty"`
	All             []IssueMatch      `json:"all,omitempty"`
	Any             []IssueMatch      `json:"any,omitempty"`
	Not             *IssueMatch       `json:"not,omitempty"`
	Severity        string            `json:"severity,omitempty"`
}

type AnalysisResult struct {
	Snapshot         *Snapshot
	RawIssues        []Issue
	Issues           []Issue
	ReportableIssues []Issue
}

func Analyze(s *Snapshot, policy Policy) (AnalysisResult, error) {
	if err := policy.Validate(); err != nil {
		return AnalysisResult{}, err
	}
	rawIssues := Validate(s, policy.PositiveLoopWhitelist)
	issues := policy.Apply(rawIssues, s)
	reportableIssues := make([]Issue, 0, len(issues))
	for _, issue := range issues {
		if policy.IsReportable(issue.Severity) {
			reportableIssues = append(reportableIssues, issue)
		}
	}
	return AnalysisResult{
		Snapshot:         s,
		RawIssues:        rawIssues,
		Issues:           issues,
		ReportableIssues: reportableIssues,
	}, nil
}

func (r AnalysisResult) SummaryCounts() map[string]int {
	return map[string]int{
		"raw_static_issue_count":        len(r.RawIssues),
		"static_issue_count":            len(r.Issues),
		"reportable_static_issue_count": len(r.ReportableIssues),
		"suppressed_static_issue_count": len(r.RawIssues) - len(r.Issues),
	}
}

func (p Policy) Validate() error {
	min := p.MinReportSeverity
	if min == "" {
		min = "info"
	}
	if _, ok := severityRanks[min]; !ok {
		return fmt.Errorf("min_report_severity must be one of: info, warning, error")
	}
	for _, rule := range p.SuppressIssueMatches {
		if err := rule.Validate(false); err != nil {
			return err
		}
	}
	for _, rule := range p.SeverityOverrideRules {
		if err := rule.Validate(true); err != nil {
			return err
		}
	}
	return nil
}

func (p Policy) Apply(issues []Issue, s *Snapshot) []Issue {
	filtered := make([]Issue, 0, len(issues))
	for _, issue := range issues {
		if p.SuppressIssueCodes[issue.Code] {
			continue
		}
		if matchesAny(p.SuppressIssueMatches, issue, s) {
			continue
		}

		if override := p.SeverityOverrides[issue.Code]; override != "" {
			normalized := strings.ToLower(override)
			if suppressionSeverities[normalized] {
				continue
			}
			issue.Severity = normalized
		}

		suppressed := false
		for _, rule := range p.SeverityOverrideRules {
			if !rule.Matches(issue, s) || rule.Severity == "" {
				continue
			}
			normalized := strings.ToLower(rule.Severity)
			if suppressionSeverities[normalized] {
				suppressed = true
				break
			}
			issue.Severity = normalized
		}
		if suppressed {
			continue
		}

		filtered = append(filtered, issue)
	}
	return filtered
}

func (p Policy) IsReportable(severity string) bool {
	minimum := "info"
	if p.MinReportSeverity != "" {
		minimum = p.MinReportSeverity
	}
	return severityRanks[severity] >= severityRanks[minimum]
}

func matchesAny(rules []IssueMatch, issue Issue, s *Snapshot) bool {
	for _, rule := range rules {
		if rule.Matches(issue, s) {
			return true
		}
	}
	return false
}

func (m IssueMatch) Validate(requireSeverity bool) error {
	if len(m.DetailsRegex) > 0 {
		for key, pattern := range m.DetailsRegex {
			if _, err := regexp.Compile(pattern); err != nil {
				return fmt.Errorf("invalid static issue match details_regex for %s: %w", key, err)
			}
		}
	}
	for _, rule := range m.All {
		if err := rule.Validate(false); err != nil {
			return err
		}
	}
	for _, rule := range m.Any {
		if err := rule.Validate(false); err != nil {
			return err
		}
	}
	if m.Not != nil {
		if err := m.Not.Validate(false); err != nil {
			return err
		}
	}
	if requireSeverity && m.Severity == "" {
		return fmt.Errorf("severity override match rules require severity")
	}
	if !m.hasMatcher() {
		return fmt.Errorf("static issue match rules require code, prototype, mod, details, details_contains, details_regex, all, any, or not")
	}
	return nil
}

func (m IssueMatch) hasMatcher() bool {
	return m.Code != "" ||
		m.Prototype != "" ||
		m.Mod != "" ||
		len(m.Details) > 0 ||
		len(m.DetailsContains) > 0 ||
		len(m.DetailsRegex) > 0 ||
		len(m.All) > 0 ||
		len(m.Any) > 0 ||
		m.Not != nil
}

func (m IssueMatch) Matches(issue Issue, s *Snapshot) bool {
	if m.Code != "" && issue.Code != m.Code {
		return false
	}
	if m.Prototype != "" && !detailsReferencePrototype(issue.Details, m.Prototype) {
		return false
	}
	if m.Mod != "" && !issueReferencesMod(issue, s, m.Mod) {
		return false
	}
	for key, expected := range m.Details {
		if !reflect.DeepEqual(issue.Details[key], expected) {
			return false
		}
	}
	for key, expected := range m.DetailsContains {
		if !detailContains(issue.Details[key], expected) {
			return false
		}
	}
	for key, pattern := range m.DetailsRegex {
		if !detailRegexMatches(issue.Details[key], pattern) {
			return false
		}
	}
	for _, rule := range m.All {
		if !rule.Matches(issue, s) {
			return false
		}
	}
	if len(m.Any) > 0 {
		matched := false
		for _, rule := range m.Any {
			if rule.Matches(issue, s) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if m.Not != nil && m.Not.Matches(issue, s) {
		return false
	}
	return true
}

func detailContains(actual any, expected any) bool {
	switch v := actual.(type) {
	case string:
		return strings.Contains(v, fmt.Sprint(expected))
	case map[string]any:
		if expectedMap, ok := expected.(map[string]any); ok {
			for key, expectedValue := range expectedMap {
				if !reflect.DeepEqual(v[key], expectedValue) {
					return false
				}
			}
			return true
		}
		_, ok := v[fmt.Sprint(expected)]
		return ok
	case []any:
		if expectedList, ok := expected.([]any); ok {
			for _, expectedValue := range expectedList {
				if !containsValue(v, expectedValue) {
					return false
				}
			}
			return true
		}
		return containsValue(v, expected)
	case []string:
		if expectedList, ok := expected.([]string); ok {
			for _, expectedValue := range expectedList {
				if !containsString(v, expectedValue) {
					return false
				}
			}
			return true
		}
		return containsString(v, fmt.Sprint(expected))
	default:
		return reflect.DeepEqual(actual, expected)
	}
}

func containsValue(values []any, expected any) bool {
	for _, value := range values {
		if reflect.DeepEqual(value, expected) {
			return true
		}
	}
	return false
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func detailRegexMatches(actual any, pattern string) bool {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	switch v := actual.(type) {
	case nil:
		return false
	case string:
		return regex.MatchString(v)
	case map[string]any:
		for key, value := range v {
			if regex.MatchString(key) || detailRegexMatches(value, pattern) {
				return true
			}
		}
		return false
	case []any:
		for _, value := range v {
			if detailRegexMatches(value, pattern) {
				return true
			}
		}
		return false
	case []string:
		for _, value := range v {
			if regex.MatchString(value) {
				return true
			}
		}
		return false
	default:
		return regex.MatchString(fmt.Sprint(actual))
	}
}

func detailsReferencePrototype(details map[string]any, prototype string) bool {
	for _, value := range details {
		if value == prototype {
			return true
		}
		switch v := value.(type) {
		case []any:
			for _, item := range v {
				if item == prototype {
					return true
				}
			}
		case []string:
			if containsString(v, prototype) {
				return true
			}
		}
	}
	return false
}

type prototypeCandidate struct {
	name     string
	sections []string
}

var detailSectionHints = map[string][]string{
	"recipe":       {"recipes"},
	"recipes":      {"recipes"},
	"item":         {"items"},
	"items":        {"items"},
	"fluid":        {"fluids"},
	"fluids":       {"fluids"},
	"entity":       {"entities"},
	"entities":     {"entities"},
	"technology":   {"technologies"},
	"technologies": {"technologies"},
	"resource":     {"resources"},
	"resources":    {"resources"},
	"module":       {"modules"},
	"modules":      {"modules"},
	"tile":         {"tiles"},
	"tiles":        {"tiles"},
	"equipment":    {"equipment"},
	"achievement":  {"achievements"},
	"achievements": {"achievements"},
	"surface":      {"surfaces"},
	"surfaces":     {"surfaces"},
}

var prototypeSections = []string{
	"recipes",
	"items",
	"fluids",
	"entities",
	"technologies",
	"resources",
	"modules",
	"crafting_categories",
	"resource_categories",
	"tiles",
	"equipment",
	"achievements",
	"surfaces",
}

var prototypeModKeys = []string{"source_mod", "owning_mod", "owner_mod", "mod_name", "mod"}

func issueReferencesMod(issue Issue, s *Snapshot, modName string) bool {
	if s == nil {
		return false
	}
	candidates := detailsPrototypeCandidates(issue.Details, nil)
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].name == candidates[j].name {
			return strings.Join(candidates[i].sections, ",") < strings.Join(candidates[j].sections, ",")
		}
		return candidates[i].name < candidates[j].name
	})
	for _, candidate := range candidates {
		if s.prototypeOwnerMod(candidate.name, candidate.sections) == modName {
			return true
		}
	}
	return false
}

func detailsPrototypeCandidates(value any, sections []string) []prototypeCandidate {
	switch v := value.(type) {
	case string:
		return []prototypeCandidate{{name: v, sections: sections}}
	case map[string]any:
		var candidates []prototypeCandidate
		for key, child := range v {
			nextSections := sections
			if hinted, ok := detailSectionHints[key]; ok {
				nextSections = hinted
			}
			candidates = append(candidates, detailsPrototypeCandidates(child, nextSections)...)
		}
		return dedupeCandidates(candidates)
	case []any:
		var candidates []prototypeCandidate
		for _, child := range v {
			candidates = append(candidates, detailsPrototypeCandidates(child, sections)...)
		}
		return dedupeCandidates(candidates)
	case []string:
		candidates := make([]prototypeCandidate, 0, len(v))
		for _, child := range v {
			candidates = append(candidates, prototypeCandidate{name: child, sections: sections})
		}
		return candidates
	default:
		return nil
	}
}

func dedupeCandidates(candidates []prototypeCandidate) []prototypeCandidate {
	if len(candidates) < 2 {
		return candidates
	}
	seen := map[string]bool{}
	out := make([]prototypeCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		key := candidate.name + "\x00" + strings.Join(candidate.sections, "\x00")
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, candidate)
	}
	return out
}

func (s *Snapshot) prototypeOwnerMod(prototypeName string, sections []string) string {
	for _, section := range sections {
		if owner := s.prototypeOwnerModInSection(section, prototypeName); owner != "" {
			return owner
		}
	}

	if owner := ownerFromAny(s.PrototypeMods[prototypeName]); owner != "" {
		return owner
	}

	for _, section := range prototypeSections {
		if owner := s.prototypeOwnerModInSection(section, prototypeName); owner != "" {
			return owner
		}
	}
	return ""
}

func (s *Snapshot) prototypeOwnerModInSection(sectionName string, prototypeName string) string {
	if owner := ownerFromMap(s.section(sectionName)[prototypeName]); owner != "" {
		return owner
	}

	sectionMods, _ := s.PrototypeMods[sectionName].(map[string]any)
	return ownerFromAny(sectionMods[prototypeName])
}

func (s *Snapshot) section(name string) map[string]map[string]any {
	switch name {
	case "recipes":
		return s.Recipes
	case "items":
		return s.Items
	case "fluids":
		return s.Fluids
	case "entities":
		return s.Entities
	case "technologies":
		return s.Technologies
	case "resources":
		return s.Resources
	case "modules":
		return s.Modules
	case "crafting_categories":
		return s.CraftingCategories
	case "resource_categories":
		return s.ResourceCategories
	case "tiles":
		return s.Tiles
	case "equipment":
		return s.Equipment
	case "achievements":
		return s.Achievements
	case "surfaces":
		return s.Surfaces
	default:
		return nil
	}
}

func ownerFromAny(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case map[string]any:
		return ownerFromMap(v)
	default:
		return ""
	}
}

func ownerFromMap(value map[string]any) string {
	for _, key := range prototypeModKeys {
		if owner, ok := value[key].(string); ok && owner != "" {
			return owner
		}
	}
	return ""
}

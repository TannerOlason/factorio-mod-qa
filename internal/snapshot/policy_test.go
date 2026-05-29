package snapshot

import "testing"

func policyTestSnapshot() *Snapshot {
	return &Snapshot{
		Items: map[string]map[string]any{
			"iron-ore":   {},
			"iron-plate": {},
		},
		Resources: map[string]map[string]any{
			"iron-ore": {
				"mineable_properties": map[string]any{
					"products": []any{map[string]any{"name": "iron-ore", "amount": float64(1)}},
				},
			},
		},
		Recipes: map[string]map[string]any{
			"iron-plate": {
				"category":    "missing-smelting",
				"ingredients": []any{map[string]any{"name": "iron-ore", "amount": float64(1)}},
				"products":    []any{map[string]any{"name": "iron-plate", "amount": float64(1)}},
			},
		},
		Entities:     map[string]map[string]any{},
		Technologies: map[string]map[string]any{},
	}
}

func TestAnalyzeAppliesIssuePolicy(t *testing.T) {
	result, err := Analyze(policyTestSnapshot(), Policy{
		SuppressIssueCodes: map[string]bool{"recipe_missing_crafting_machine": true},
		SeverityOverrides:  map[string]string{"item_without_use": "warning"},
		MinReportSeverity:  "warning",
	})
	if err != nil {
		t.Fatal(err)
	}

	counts := result.SummaryCounts()
	if counts["raw_static_issue_count"] != 2 ||
		counts["static_issue_count"] != 1 ||
		counts["reportable_static_issue_count"] != 1 ||
		counts["suppressed_static_issue_count"] != 1 {
		t.Fatalf("unexpected summary counts: %#v", counts)
	}
	if result.Issues[0].Code != "item_without_use" || result.Issues[0].Severity != "warning" {
		t.Fatalf("unexpected filtered issue: %#v", result.Issues[0])
	}
}

func TestAnalyzeCanSuppressViaSeverityOverride(t *testing.T) {
	result, err := Analyze(policyTestSnapshot(), Policy{
		SeverityOverrides: map[string]string{
			"recipe_missing_crafting_machine": "ignore",
			"item_without_use":                "suppress",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.RawIssues) == 0 {
		t.Fatal("expected raw issues")
	}
	if len(result.Issues) != 0 || len(result.ReportableIssues) != 0 {
		t.Fatalf("expected all issues to be suppressed: %#v", result.Issues)
	}
}

func TestAnalyzeMatchesExactContainsRegexAndBooleanRules(t *testing.T) {
	snap := policyTestSnapshot()
	snap.Recipes["broken-widget"] = map[string]any{
		"category":    "missing-widget-crafting",
		"ingredients": []any{map[string]any{"name": "missing-widget-part", "amount": float64(1)}},
		"products":    []any{map[string]any{"name": "iron-plate", "amount": float64(1)}},
	}

	result, err := Analyze(snap, Policy{
		SuppressIssueMatches: []IssueMatch{
			{
				All: []IssueMatch{
					{Code: "recipe_missing_crafting_machine"},
					{Any: []IssueMatch{
						{Details: map[string]any{"recipe": "iron-plate"}},
						{DetailsRegex: map[string]string{"category": "^missing-.+-crafting$"}},
					}},
					{Not: &IssueMatch{Details: map[string]any{"category": "allowed-smelting"}}},
				},
			},
		},
		SeverityOverrideRules: []IssueMatch{
			{
				Code:            "recipe_missing_ingredient_prototype",
				DetailsContains: map[string]any{"ingredients": "missing-widget-part"},
				Severity:        "warning",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	byCode := issuesByCode(result.Issues)
	if _, ok := byCode["recipe_missing_crafting_machine"]; ok {
		t.Fatalf("expected missing-machine issues to be suppressed: %#v", result.Issues)
	}
	if byCode["recipe_missing_ingredient_prototype"].Severity != "warning" {
		t.Fatalf("expected ingredient issue severity override: %#v", byCode["recipe_missing_ingredient_prototype"])
	}
}

func TestAnalyzeMatchesPolicyByModProvenance(t *testing.T) {
	snap := policyTestSnapshot()
	snap.Recipes["iron-plate"]["source_mod"] = "recipe-mod"
	snap.PrototypeMods = map[string]any{
		"items": map[string]any{"iron-plate": "item-mod"},
	}

	result, err := Analyze(snap, Policy{
		SuppressIssueMatches: []IssueMatch{
			{Code: "recipe_missing_crafting_machine", Mod: "recipe-mod"},
		},
		SeverityOverrideRules: []IssueMatch{
			{Code: "item_without_use", Mod: "item-mod", Severity: "warning"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Issues) != 1 {
		t.Fatalf("expected one filtered issue: %#v", result.Issues)
	}
	if result.Issues[0].Code != "item_without_use" || result.Issues[0].Severity != "warning" {
		t.Fatalf("unexpected issue after mod policy: %#v", result.Issues[0])
	}
}

func TestPolicyRejectsInvalidRegex(t *testing.T) {
	_, err := Analyze(policyTestSnapshot(), Policy{
		SuppressIssueMatches: []IssueMatch{
			{Code: "recipe_missing_crafting_machine", DetailsRegex: map[string]string{"category": "["}},
		},
	})
	if err == nil {
		t.Fatal("expected invalid regex error")
	}
}

func issuesByCode(issues []Issue) map[string]Issue {
	out := map[string]Issue{}
	for _, issue := range issues {
		out[issue.Code] = issue
	}
	return out
}

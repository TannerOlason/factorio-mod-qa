package snapshot

import (
	"path/filepath"
	"testing"
)

func TestValidateBrokenFixtureSnapshot(t *testing.T) {
	snap, err := Load(filepath.Join("..", "..", "fixtures", "prototype_snapshots", "qa_broken_mod.json"))
	if err != nil {
		t.Fatal(err)
	}

	issues := Validate(snap, nil)
	codes := map[string]bool{}
	for _, issue := range issues {
		codes[issue.Code] = true
	}

	for _, code := range []string{
		"recipe_missing_ingredient_prototype",
		"recipe_missing_crafting_machine",
		"technology_missing_prerequisite",
		"technology_unlocks_missing_recipe",
		"technology_unreachable_science_pack",
		"positive_output_loop",
	} {
		if !codes[code] {
			t.Fatalf("expected issue code %s in %#v", code, codes)
		}
	}
}

func TestPositiveLoopWhitelist(t *testing.T) {
	snap := &Snapshot{
		Recipes: map[string]map[string]any{
			"loop": {
				"category": "crafting",
				"ingredients": []any{
					map[string]any{"name": "iron-plate", "amount": float64(1)},
				},
				"products": []any{
					map[string]any{"name": "iron-plate", "amount": float64(2)},
				},
			},
		},
		Items: map[string]map[string]any{
			"iron-plate": {},
		},
		Entities: map[string]map[string]any{
			"assembler": {
				"crafting_categories": []any{"crafting"},
			},
		},
	}

	issues := Validate(snap, map[string]bool{"loop": true})
	for _, issue := range issues {
		if issue.Code == "positive_output_loop" {
			t.Fatalf("positive_output_loop was not whitelisted: %#v", issues)
		}
	}
}

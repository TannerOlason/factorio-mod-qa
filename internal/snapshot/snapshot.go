package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

type Snapshot struct {
	SchemaVersion      int                       `json:"schema_version"`
	FactorioVersion    string                    `json:"factorio_version,omitempty"`
	ActiveMods         map[string]string         `json:"active_mods,omitempty"`
	Recipes            map[string]map[string]any `json:"recipes,omitempty"`
	Items              map[string]map[string]any `json:"items,omitempty"`
	Fluids             map[string]map[string]any `json:"fluids,omitempty"`
	Entities           map[string]map[string]any `json:"entities,omitempty"`
	Technologies       map[string]map[string]any `json:"technologies,omitempty"`
	Resources          map[string]map[string]any `json:"resources,omitempty"`
	Modules            map[string]map[string]any `json:"modules,omitempty"`
	CraftingCategories map[string]map[string]any `json:"crafting_categories,omitempty"`
	ResourceCategories map[string]map[string]any `json:"resource_categories,omitempty"`
	Tiles              map[string]map[string]any `json:"tiles,omitempty"`
	Equipment          map[string]map[string]any `json:"equipment,omitempty"`
	Achievements       map[string]map[string]any `json:"achievements,omitempty"`
	Surfaces           map[string]map[string]any `json:"surfaces,omitempty"`
	PrototypeMods      map[string]any            `json:"prototype_mods,omitempty"`
}

type Issue struct {
	Code     string         `json:"code"`
	Severity string         `json:"severity"`
	Title    string         `json:"title"`
	Details  map[string]any `json:"details,omitempty"`
}

func Load(path string) (*Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, err
	}
	snap.normalize()
	return &snap, nil
}

func Write(path string, snap *Snapshot) error {
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func Decode(data []byte) (*Snapshot, error) {
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, err
	}
	snap.normalize()
	return &snap, nil
}

func (s *Snapshot) normalize() {
	if s.ActiveMods == nil {
		s.ActiveMods = map[string]string{}
	}
	if s.Recipes == nil {
		s.Recipes = map[string]map[string]any{}
	}
	if s.Items == nil {
		s.Items = map[string]map[string]any{}
	}
	if s.Fluids == nil {
		s.Fluids = map[string]map[string]any{}
	}
	if s.Entities == nil {
		s.Entities = map[string]map[string]any{}
	}
	if s.Technologies == nil {
		s.Technologies = map[string]map[string]any{}
	}
	if s.Resources == nil {
		s.Resources = map[string]map[string]any{}
	}
	if s.Modules == nil {
		s.Modules = map[string]map[string]any{}
	}
	if s.CraftingCategories == nil {
		s.CraftingCategories = map[string]map[string]any{}
	}
	if s.ResourceCategories == nil {
		s.ResourceCategories = map[string]map[string]any{}
	}
	if s.Tiles == nil {
		s.Tiles = map[string]map[string]any{}
	}
	if s.Equipment == nil {
		s.Equipment = map[string]map[string]any{}
	}
	if s.Achievements == nil {
		s.Achievements = map[string]map[string]any{}
	}
	if s.Surfaces == nil {
		s.Surfaces = map[string]map[string]any{}
	}
	if s.PrototypeMods == nil {
		s.PrototypeMods = map[string]any{}
	}
}

func Validate(s *Snapshot, positiveLoopWhitelist map[string]bool) []Issue {
	s.normalize()
	var issues []Issue
	ingredientByRecipe := map[string]map[string]bool{}
	productByRecipe := map[string]map[string]bool{}
	producers := map[string]map[string]bool{}
	consumers := map[string]map[string]bool{}
	machinesByCategory := map[string]map[string]bool{}

	for name, recipe := range s.Recipes {
		ingredients := namesFromList(recipe["ingredients"])
		products := namesFromList(recipe["products"])
		ingredientByRecipe[name] = ingredients
		productByRecipe[name] = products
		for ingredient := range ingredients {
			addSet(consumers, ingredient, name)
		}
		for product := range products {
			addSet(producers, product, name)
		}
	}

	for entityName, entity := range s.Entities {
		for _, category := range stringList(entity["crafting_categories"]) {
			addSet(machinesByCategory, category, entityName)
		}
	}

	for resourceName, resource := range s.Resources {
		mineable, _ := resource["mineable_properties"].(map[string]any)
		for product := range namesFromList(mineable["products"]) {
			addSet(producers, product, "resource:"+resourceName)
		}
	}

	knownMaterials := map[string]bool{}
	for name := range s.Items {
		knownMaterials[name] = true
	}
	for name := range s.Fluids {
		knownMaterials[name] = true
	}
	for resource := range s.Resources {
		mineable, _ := s.Resources[resource]["mineable_properties"].(map[string]any)
		for product := range namesFromList(mineable["products"]) {
			knownMaterials[product] = true
		}
	}

	for recipeName, ingredients := range ingredientByRecipe {
		missing := sortedMissing(ingredients, knownMaterials)
		if len(missing) > 0 {
			issues = append(issues, Issue{
				Code:     "recipe_missing_ingredient_prototype",
				Severity: "error",
				Title:    fmt.Sprintf("Recipe %s references unknown ingredients", recipeName),
				Details:  map[string]any{"recipe": recipeName, "ingredients": missing},
			})
		}
		category, _ := s.Recipes[recipeName]["category"].(string)
		if category != "" && len(machinesByCategory[category]) == 0 {
			issues = append(issues, Issue{
				Code:     "recipe_missing_crafting_machine",
				Severity: "error",
				Title:    fmt.Sprintf("Recipe %s has no crafting machine", recipeName),
				Details:  map[string]any{"recipe": recipeName, "category": category},
			})
		}
	}

	for fluidName := range s.Fluids {
		hasSource := len(producers[fluidName]) > 0
		hasSink := len(consumers[fluidName]) > 0
		if !hasSource || !hasSink {
			issues = append(issues, Issue{
				Code:     "fluid_source_sink_gap",
				Severity: "warning",
				Title:    fmt.Sprintf("Fluid %s is missing a source or sink", fluidName),
				Details:  map[string]any{"fluid": fluidName, "has_source": hasSource, "has_sink": hasSink},
			})
		}
	}

	for itemName, item := range s.Items {
		_, hasSource := producers[itemName]
		_, hasSink := consumers[itemName]
		placeResult, _ := item["place_result"].(string)
		if hasSource && !hasSink && placeResult == "" {
			issues = append(issues, Issue{
				Code:     "item_without_use",
				Severity: "info",
				Title:    fmt.Sprintf("Item %s has no recipe consumer or place result", itemName),
				Details:  map[string]any{"item": itemName},
			})
		}
	}

	for techName, tech := range s.Technologies {
		missingPrereqs := missingStrings(stringList(tech["prerequisites"]), s.Technologies)
		if len(missingPrereqs) > 0 {
			issues = append(issues, Issue{
				Code:     "technology_missing_prerequisite",
				Severity: "error",
				Title:    fmt.Sprintf("Technology %s references missing prerequisites", techName),
				Details:  map[string]any{"technology": techName, "prerequisites": missingPrereqs},
			})
		}
		var missingScience []string
		for pack := range namesFromList(tech["research_unit_ingredients"]) {
			if len(producers[pack]) == 0 {
				if _, ok := s.Items[pack]; !ok {
					missingScience = append(missingScience, pack)
				}
			}
		}
		sort.Strings(missingScience)
		if len(missingScience) > 0 {
			issues = append(issues, Issue{
				Code:     "technology_unreachable_science_pack",
				Severity: "error",
				Title:    fmt.Sprintf("Technology %s requires unreachable science packs", techName),
				Details:  map[string]any{"technology": techName, "science_packs": missingScience},
			})
		}
		var missingRecipes []string
		for _, effect := range objectList(tech["effects"]) {
			if effect["type"] != "unlock-recipe" {
				continue
			}
			recipe, _ := effect["recipe"].(string)
			if recipe != "" {
				if _, ok := s.Recipes[recipe]; !ok {
					missingRecipes = append(missingRecipes, recipe)
				}
			}
		}
		sort.Strings(missingRecipes)
		if len(missingRecipes) > 0 {
			issues = append(issues, Issue{
				Code:     "technology_unlocks_missing_recipe",
				Severity: "error",
				Title:    fmt.Sprintf("Technology %s unlocks missing recipes", techName),
				Details:  map[string]any{"technology": techName, "recipes": missingRecipes},
			})
		}
	}

	for recipeName, ingredients := range ingredientByRecipe {
		if positiveLoopWhitelist[recipeName] {
			continue
		}
		var netPositive []string
		for material := range ingredients {
			if !productByRecipe[recipeName][material] {
				continue
			}
			if recipeAmount(s.Recipes[recipeName]["products"], material, true) > recipeAmount(s.Recipes[recipeName]["ingredients"], material, false) {
				netPositive = append(netPositive, material)
			}
		}
		sort.Strings(netPositive)
		if len(netPositive) > 0 {
			issues = append(issues, Issue{
				Code:     "positive_output_loop",
				Severity: "warning",
				Title:    fmt.Sprintf("Recipe %s has positive same-material output", recipeName),
				Details:  map[string]any{"recipe": recipeName, "materials": netPositive},
			})
		}
	}

	return issues
}

func addSet(m map[string]map[string]bool, key, value string) {
	if key == "" || value == "" {
		return
	}
	if m[key] == nil {
		m[key] = map[string]bool{}
	}
	m[key][value] = true
}

func namesFromList(value any) map[string]bool {
	names := map[string]bool{}
	for _, obj := range objectList(value) {
		name, _ := obj["name"].(string)
		if name != "" {
			names[name] = true
		}
	}
	return names
}

func objectList(value any) []map[string]any {
	items, _ := value.([]any)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		obj, _ := item.(map[string]any)
		if obj != nil {
			out = append(out, obj)
		}
	}
	return out
}

func stringList(value any) []string {
	items, _ := value.([]any)
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func sortedMissing(values map[string]bool, known map[string]bool) []string {
	var missing []string
	for value := range values {
		if !known[value] {
			missing = append(missing, value)
		}
	}
	sort.Strings(missing)
	return missing
}

func missingStrings[T any](values []string, known map[string]T) []string {
	var missing []string
	for _, value := range values {
		if _, ok := known[value]; !ok {
			missing = append(missing, value)
		}
	}
	sort.Strings(missing)
	return missing
}

func recipeAmount(value any, material string, withProbability bool) float64 {
	var total float64
	for _, obj := range objectList(value) {
		name, _ := obj["name"].(string)
		if name != material {
			continue
		}
		amount, _ := number(obj["amount"])
		if withProbability {
			probability, ok := number(obj["probability"])
			if !ok {
				probability = 1
			}
			amount *= probability
		}
		total += amount
	}
	return total
}

func number(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	default:
		return 0, false
	}
}

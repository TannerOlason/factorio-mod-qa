package qa

import (
	"context"
	"fmt"

	"factorio-mod-qa/internal/snapshot"
)

type InventoryAbuseScenario struct {
	Entity string
	Item   string
	Count  int

	unitNumber float64
	inserted   map[string]float64
	mined      bool
	returned   map[string]float64
	skipped    string
}

func NewInventoryAbuseScenario() *InventoryAbuseScenario {
	return &InventoryAbuseScenario{
		Entity: "steel-chest",
		Item:   "iron-plate",
		Count:  10,
	}
}

func (s *InventoryAbuseScenario) Name() string {
	return "inventory-abuse"
}

func (s *InventoryAbuseScenario) Setup(ctx context.Context, session *Session) error {
	if session.Snapshot == nil {
		return fmt.Errorf("scenario %s requires a prototype snapshot", s.Name())
	}
	if _, ok := session.Snapshot.Entities[s.Entity]; !ok {
		if _, ok := session.Snapshot.Entities["wooden-chest"]; ok {
			s.Entity = "wooden-chest"
		} else {
			s.skipped = "no supported chest entity present"
			session.Record("setup", "skip_missing_chest", nil, map[string]any{"reason": s.skipped}, nil)
			return nil
		}
	}
	if _, ok := session.Snapshot.Items[s.Item]; !ok {
		s.skipped = "iron-plate item prototype not present"
		session.Record("setup", "skip_missing_item", nil, map[string]any{"reason": s.skipped, "item": s.Item}, nil)
		return nil
	}

	var placed struct {
		UnitNumber float64 `json:"unit_number"`
	}
	if err := session.Dispatch("place_entity", map[string]any{
		"name":     s.Entity,
		"surface":  "nauvis",
		"position": map[string]float64{"x": 16, "y": 16},
		"force":    "player",
	}, &placed); err != nil {
		return err
	}
	if placed.UnitNumber == 0 {
		return fmt.Errorf("placed %s without unit_number", s.Entity)
	}
	s.unitNumber = placed.UnitNumber
	return nil
}

func (s *InventoryAbuseScenario) Run(ctx context.Context, session *Session) error {
	if s.skipped != "" {
		return nil
	}
	var inserted struct {
		Inserted map[string]float64 `json:"inserted"`
	}
	if err := session.Dispatch("insert_items", map[string]any{
		"unit_number": s.unitNumber,
		"items":       map[string]int{s.Item: s.Count},
	}, &inserted); err != nil {
		return err
	}
	s.inserted = inserted.Inserted

	var mined struct {
		Mined     bool               `json:"mined"`
		Inventory map[string]float64 `json:"inventory"`
	}
	if err := session.Dispatch("mine_entity_to_inventory", map[string]any{
		"unit_number": s.unitNumber,
		"force":       "player",
		"buffer_size": 100,
	}, &mined); err != nil {
		return err
	}
	s.mined = mined.Mined
	s.returned = mined.Inventory
	return nil
}

func (s *InventoryAbuseScenario) Check(ctx context.Context, session *Session) Result {
	if s.skipped != "" {
		return Result{Observations: map[string]any{"skipped": true, "reason": s.skipped}}
	}

	expected := map[string]float64{}
	for name, count := range s.inserted {
		expected[name] += count
	}
	for name, count := range expectedMineableProducts(session.Snapshot, s.Entity) {
		expected[name] += count
	}

	var issues []snapshot.Issue
	if !s.mined {
		issues = append(issues, snapshot.Issue{
			Code:     "inventory_abuse_mine_failed",
			Severity: "warning",
			Title:    fmt.Sprintf("Inventory abuse scenario could not mine %s", s.Entity),
			Details:  map[string]any{"scenario": s.Name(), "entity": s.Entity},
		})
	}
	for name, returned := range s.returned {
		if returned > expected[name] {
			issues = append(issues, snapshot.Issue{
				Code:     "inventory_positive_delta",
				Severity: "error",
				Title:    fmt.Sprintf("Inventory abuse returned extra %s", name),
				Details: map[string]any{
					"scenario": s.Name(),
					"entity":   s.Entity,
					"item":     name,
					"expected": expected[name],
					"returned": returned,
				},
			})
		}
	}
	return Result{
		Issues: issues,
		Observations: map[string]any{
			"entity":   s.Entity,
			"inserted": s.inserted,
			"expected": expected,
			"returned": s.returned,
			"mined":    s.mined,
		},
	}
}

func (s *InventoryAbuseScenario) Cleanup(ctx context.Context, session *Session) error {
	if s.unitNumber == 0 || s.mined {
		return nil
	}
	var removed map[string]any
	return session.Dispatch("remove_entity", map[string]any{"unit_number": s.unitNumber}, &removed)
}

func expectedMineableProducts(snap *snapshot.Snapshot, entity string) map[string]float64 {
	expected := map[string]float64{}
	if snap == nil {
		return expected
	}
	proto := snap.Entities[entity]
	mineable, _ := proto["mineable_properties"].(map[string]any)
	for _, product := range objectList(mineable["products"]) {
		name, _ := product["name"].(string)
		if name == "" {
			continue
		}
		amount := number(product["amount"])
		if amount == 0 {
			amount = number(product["amount_min"])
		}
		if amount == 0 {
			amount = 1
		}
		expected[name] += amount
	}
	return expected
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

func number(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	default:
		return 0
	}
}

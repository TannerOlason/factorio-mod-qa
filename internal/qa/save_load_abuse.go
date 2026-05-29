package qa

import (
	"context"
	"fmt"

	"factorio-mod-qa/internal/snapshot"
)

type SaveLoadAbuseScenario struct {
	Entity string
	Item   string
	Count  int

	unitNumber      float64
	saveName        string
	beforeInventory map[string]float64
	afterInventory  map[string]float64
	mined           bool
	returned        map[string]float64
	inserted        map[string]float64
	skipped         string
}

func NewSaveLoadAbuseScenario() *SaveLoadAbuseScenario {
	return &SaveLoadAbuseScenario{
		Entity: "steel-chest",
		Item:   "iron-plate",
		Count:  10,
	}
}

func (s *SaveLoadAbuseScenario) Name() string {
	return "save-load-abuse"
}

func (s *SaveLoadAbuseScenario) Setup(ctx context.Context, session *Session) error {
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
		"position": map[string]float64{"x": 20, "y": 20},
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

func (s *SaveLoadAbuseScenario) Run(ctx context.Context, session *Session) error {
	if s.skipped != "" {
		return nil
	}
	var inserted struct {
		Inserted  map[string]float64 `json:"inserted"`
		Inventory map[string]float64 `json:"inventory"`
	}
	if err := session.Dispatch("insert_items", map[string]any{
		"unit_number": s.unitNumber,
		"items":       map[string]int{s.Item: s.Count},
	}, &inserted); err != nil {
		return err
	}
	s.inserted = inserted.Inserted
	s.beforeInventory = inserted.Inventory

	s.saveName = "fmqa-" + sanitizeName(session.RunID) + "-save-load-abuse"
	var saveResult map[string]any
	if err := session.Dispatch("save", map[string]any{"name": s.saveName}, &saveResult); err != nil {
		return err
	}
	if err := session.Restart(ctx, s.saveName); err != nil {
		return err
	}

	var after struct {
		Inventory map[string]float64 `json:"inventory"`
	}
	if err := session.Dispatch("read_entity_inventory", map[string]any{
		"unit_number": s.unitNumber,
	}, &after); err != nil {
		return err
	}
	s.afterInventory = after.Inventory

	var mined struct {
		Mined     bool               `json:"mined"`
		Inventory map[string]float64 `json:"inventory"`
	}
	if err := session.Dispatch("mine_entity_to_inventory", map[string]any{
		"unit_number": s.unitNumber,
		"buffer_size": 100,
	}, &mined); err != nil {
		return err
	}
	s.mined = mined.Mined
	s.returned = mined.Inventory
	return nil
}

func (s *SaveLoadAbuseScenario) Check(ctx context.Context, session *Session) Result {
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
	for name, before := range s.beforeInventory {
		if s.afterInventory[name] != before {
			issues = append(issues, snapshot.Issue{
				Code:     "save_load_inventory_mismatch",
				Severity: "error",
				Title:    fmt.Sprintf("Inventory count for %s changed across save/load", name),
				Details: map[string]any{
					"scenario": s.Name(),
					"entity":   s.Entity,
					"item":     name,
					"before":   before,
					"after":    s.afterInventory[name],
				},
			})
		}
	}
	if !s.mined {
		issues = append(issues, snapshot.Issue{
			Code:     "save_load_mine_failed",
			Severity: "warning",
			Title:    fmt.Sprintf("Save/load abuse scenario could not mine %s after reload", s.Entity),
			Details:  map[string]any{"scenario": s.Name(), "entity": s.Entity},
		})
	}
	for name, returned := range s.returned {
		if returned > expected[name] {
			issues = append(issues, snapshot.Issue{
				Code:     "save_load_positive_delta",
				Severity: "error",
				Title:    fmt.Sprintf("Save/load abuse returned extra %s", name),
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
			"entity":           s.Entity,
			"save":             s.saveName,
			"before_inventory": s.beforeInventory,
			"after_inventory":  s.afterInventory,
			"expected":         expected,
			"returned":         s.returned,
			"mined":            s.mined,
		},
	}
}

func (s *SaveLoadAbuseScenario) Cleanup(ctx context.Context, session *Session) error {
	if s.unitNumber == 0 || s.mined {
		return nil
	}
	var removed map[string]any
	return session.Dispatch("remove_entity", map[string]any{"unit_number": s.unitNumber}, &removed)
}

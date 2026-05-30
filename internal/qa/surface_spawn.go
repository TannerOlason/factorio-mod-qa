package qa

import (
	"context"
	"fmt"

	"factorio-mod-qa/internal/snapshot"
)

type SurfaceSpawnScenario struct {
	Entity string

	surface    string
	unitNumber float64
	position   map[string]float64
	mined      bool
	returned   map[string]float64
	skipped    string
}

func NewSurfaceSpawnScenario() *SurfaceSpawnScenario {
	return &SurfaceSpawnScenario{Entity: "steel-chest"}
}

func (s *SurfaceSpawnScenario) Name() string {
	return "surface-spawn"
}

func (s *SurfaceSpawnScenario) Setup(ctx context.Context, session *Session) error {
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
	s.surface = "fmqa-" + sanitizeName(session.RunID) + "-surface-spawn"
	var created map[string]any
	return session.Dispatch("create_surface", map[string]any{
		"name":         s.surface,
		"position":     map[string]float64{"x": 0, "y": 0},
		"chunk_radius": 2,
	}, &created)
}

func (s *SurfaceSpawnScenario) Run(ctx context.Context, session *Session) error {
	if s.skipped != "" {
		return nil
	}
	var found struct {
		Position map[string]float64 `json:"position"`
	}
	if err := session.Dispatch("find_buildable_position", map[string]any{
		"surface":   s.surface,
		"entity":    s.Entity,
		"position":  map[string]float64{"x": 0, "y": 0},
		"radius":    64,
		"precision": 1,
	}, &found); err != nil {
		return err
	}
	s.position = found.Position
	if len(s.position) == 0 {
		return nil
	}

	var placed struct {
		UnitNumber float64 `json:"unit_number"`
	}
	if err := session.Dispatch("place_entity", map[string]any{
		"name":     s.Entity,
		"surface":  s.surface,
		"position": s.position,
		"force":    "player",
	}, &placed); err != nil {
		return err
	}
	if placed.UnitNumber == 0 {
		return fmt.Errorf("placed %s without unit_number on %s", s.Entity, s.surface)
	}
	s.unitNumber = placed.UnitNumber

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

func (s *SurfaceSpawnScenario) Check(ctx context.Context, session *Session) Result {
	if s.skipped != "" {
		return Result{Observations: map[string]any{"skipped": true, "reason": s.skipped}}
	}

	var issues []snapshot.Issue
	if len(s.position) == 0 {
		issues = append(issues, snapshot.Issue{
			Code:     "surface_spawn_blocked",
			Severity: "error",
			Title:    fmt.Sprintf("No buildable %s position found near spawn on %s", s.Entity, s.surface),
			Details:  map[string]any{"scenario": s.Name(), "surface": s.surface, "entity": s.Entity},
		})
	}
	if len(s.position) > 0 && !s.mined {
		issues = append(issues, snapshot.Issue{
			Code:     "surface_spawn_mine_failed",
			Severity: "warning",
			Title:    fmt.Sprintf("Surface spawn scenario could not mine %s on %s", s.Entity, s.surface),
			Details:  map[string]any{"scenario": s.Name(), "surface": s.surface, "entity": s.Entity},
		})
	}
	expected := expectedMineableProducts(session.Snapshot, s.Entity)
	for name, returned := range s.returned {
		if returned > expected[name] {
			issues = append(issues, snapshot.Issue{
				Code:     "surface_spawn_positive_delta",
				Severity: "error",
				Title:    fmt.Sprintf("Surface spawn scenario returned extra %s", name),
				Details: map[string]any{
					"scenario": s.Name(),
					"surface":  s.surface,
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
			"surface":  s.surface,
			"entity":   s.Entity,
			"position": s.position,
			"mined":    s.mined,
			"expected": expected,
			"returned": s.returned,
		},
	}
}

func (s *SurfaceSpawnScenario) Cleanup(ctx context.Context, session *Session) error {
	if s.unitNumber != 0 && !s.mined {
		var removed map[string]any
		if err := session.Dispatch("remove_entity", map[string]any{"unit_number": s.unitNumber}, &removed); err != nil {
			return err
		}
	}
	if s.surface == "" {
		return nil
	}
	var deleted map[string]any
	return session.Dispatch("delete_surface", map[string]any{"name": s.surface}, &deleted)
}

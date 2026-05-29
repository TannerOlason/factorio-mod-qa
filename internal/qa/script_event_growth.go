package qa

import (
	"context"
	"fmt"
	"time"

	"factorio-mod-qa/internal/snapshot"
)

type ScriptEventGrowthScenario struct {
	Entity    string
	Count     int
	Threshold float64
	Wait      time.Duration

	enabled bool
	counts  map[string]float64
}

func NewScriptEventGrowthScenario() *ScriptEventGrowthScenario {
	return &ScriptEventGrowthScenario{
		Entity:    "qa-ticking-machine",
		Count:     12,
		Threshold: 100,
		Wait:      5 * time.Second,
	}
}

func (s *ScriptEventGrowthScenario) Name() string {
	return "script-event-growth"
}

func (s *ScriptEventGrowthScenario) Setup(ctx context.Context, session *Session) error {
	if session.Snapshot == nil {
		return fmt.Errorf("scenario %s requires a prototype snapshot", s.Name())
	}
	if _, ok := session.Snapshot.Entities[s.Entity]; !ok {
		s.enabled = false
		session.Record("setup", "skip_missing_entity", map[string]any{"entity": s.Entity}, nil, nil)
		return nil
	}
	s.enabled = true
	var result map[string]any
	return session.Dispatch("reset_script_event_counts", nil, &result)
}

func (s *ScriptEventGrowthScenario) Run(ctx context.Context, session *Session) error {
	if !s.enabled {
		return nil
	}
	payload := map[string]any{
		"surface":  "nauvis",
		"entities": scriptStressEntities(s.Entity, s.Count),
	}
	var placed map[string]any
	if err := session.Dispatch("place_entities_batch", payload, &placed); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(s.Wait):
	}
	counts := map[string]float64{}
	if err := session.Dispatch("script_event_counts", nil, &counts); err != nil {
		return err
	}
	s.counts = counts
	return nil
}

func (s *ScriptEventGrowthScenario) Check(ctx context.Context, session *Session) Result {
	if !s.enabled {
		return Result{Observations: map[string]any{"skipped": true, "reason": "entity not present", "entity": s.Entity}}
	}
	var issues []snapshot.Issue
	for name, count := range s.counts {
		if count >= s.Threshold {
			issues = append(issues, snapshot.Issue{
				Code:     "script_event_growth",
				Severity: "warning",
				Title:    fmt.Sprintf("Script event counter %s grew during stress probe", name),
				Details:  map[string]any{"counter": name, "count": count, "entity": s.Entity, "scenario": s.Name()},
			})
		}
	}
	return Result{
		Issues: issues,
		Observations: map[string]any{
			"counts":    s.counts,
			"entity":    s.Entity,
			"threshold": s.Threshold,
		},
	}
}

func (s *ScriptEventGrowthScenario) Cleanup(ctx context.Context, session *Session) error {
	return nil
}

func scriptStressEntities(name string, count int) []map[string]any {
	entities := make([]map[string]any, 0, count)
	for i := 0; i < count; i++ {
		entities = append(entities, map[string]any{
			"name": name,
			"position": map[string]float64{
				"x": float64(i % 4),
				"y": float64(i / 4),
			},
			"force": "player",
		})
	}
	return entities
}

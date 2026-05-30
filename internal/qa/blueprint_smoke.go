package qa

import (
	"context"
	"fmt"
	"time"

	"factorio-mod-qa/internal/blueprint"
	"factorio-mod-qa/internal/snapshot"
)

type BlueprintSmokeScenario struct {
	Document *blueprint.Document
	Options  BlueprintOptions

	surface       string
	ghostResult   blueprintSmokeResult
	instantResult blueprintSmokeResult
	instantCopies []blueprintSmokeResult
	observed      trackedEntitiesResult
	tickWindow    tickWindowObservation
	placementMS   int64
}

type BlueprintOptions struct {
	Copies     int
	Spacing    int
	TickWindow int
}

type blueprintSmokeResult struct {
	Mode             string                   `json:"mode"`
	Surface          string                   `json:"surface"`
	ExpectedEntities int                      `json:"expected_entities"`
	CreatedCount     int                      `json:"created_count"`
	Created          []blueprintPlacedEntity  `json:"created,omitempty"`
	Missing          []blueprintMissingEntity `json:"missing,omitempty"`
	Configured       []blueprintConfigured    `json:"configured,omitempty"`
}

type blueprintPlacedEntity struct {
	Name            string         `json:"name"`
	UnitNumber      int            `json:"unit_number,omitempty"`
	Position        map[string]any `json:"position,omitempty"`
	BlueprintRecipe string         `json:"blueprint_recipe,omitempty"`
	ActualRecipe    string         `json:"actual_recipe,omitempty"`
	RecipeLocked    *bool          `json:"recipe_locked,omitempty"`
	Inventory       map[string]int `json:"inventory,omitempty"`
	ModuleInventory map[string]int `json:"module_inventory,omitempty"`
}

type blueprintMissingEntity struct {
	Name   string `json:"name"`
	Reason string `json:"reason,omitempty"`
}

type blueprintConfigured struct {
	EntityNumber int      `json:"entity_number,omitempty"`
	Name         string   `json:"name"`
	Fields       []string `json:"fields,omitempty"`
	Recipe       string   `json:"recipe,omitempty"`
}

type trackedEntitiesResult struct {
	Entities []blueprintPlacedEntity `json:"entities,omitempty"`
}

type runtimeSummary struct {
	Tick int64 `json:"tick"`
}

type tickWindowObservation struct {
	StartTick      int64 `json:"start_tick"`
	EndTick        int64 `json:"end_tick"`
	RequestedTicks int   `json:"requested_ticks"`
	ElapsedMS      int64 `json:"elapsed_ms"`
	Polls          int   `json:"polls"`
}

func NewBlueprintSmokeScenario(doc *blueprint.Document, options BlueprintOptions) *BlueprintSmokeScenario {
	if options.Copies <= 0 {
		options.Copies = 1
	}
	if options.Spacing <= 0 {
		options.Spacing = 12
	}
	if options.TickWindow <= 0 {
		options.TickWindow = 120
	}
	return &BlueprintSmokeScenario{Document: doc, Options: options}
}

func (s *BlueprintSmokeScenario) Name() string {
	return "blueprint-smoke"
}

func (s *BlueprintSmokeScenario) Setup(ctx context.Context, session *Session) error {
	if s.Document == nil || s.Document.Raw == nil {
		return fmt.Errorf("blueprint-smoke requires --blueprint")
	}
	s.surface = "fmqa-blueprint-" + sanitizeName(session.RunID)
	if s.surface == "fmqa-blueprint-" {
		s.surface = "fmqa-blueprint-smoke"
	}
	var created struct {
		Name string `json:"name"`
	}
	return session.Dispatch("create_surface", map[string]any{
		"name":         s.surface,
		"chunk_radius": chunkRadiusForCopies(s.Options.Copies, s.Options.Spacing),
		"position":     []float64{0, 0},
	}, &created)
}

func (s *BlueprintSmokeScenario) Run(ctx context.Context, session *Session) error {
	payload := map[string]any{
		"blueprint": s.Document.Raw,
		"surface":   s.surface,
		"force":     "player",
	}

	ghostPayload := cloneStringAny(payload)
	ghostPayload["mode"] = "ghost"
	ghostPayload["position"] = []float64{0, 0}
	if err := session.Dispatch("blueprint_smoke", ghostPayload, &s.ghostResult); err != nil {
		return err
	}

	start := time.Now()
	s.instantCopies = make([]blueprintSmokeResult, 0, s.Options.Copies)
	for i := 0; i < s.Options.Copies; i++ {
		instantPayload := cloneStringAny(payload)
		instantPayload["mode"] = "instant"
		instantPayload["position"] = copyPosition(i, s.Options.Spacing)
		var result blueprintSmokeResult
		if err := session.Dispatch("blueprint_smoke", instantPayload, &result); err != nil {
			return err
		}
		if i == 0 {
			s.instantResult = result
		}
		s.instantCopies = append(s.instantCopies, result)
	}
	s.placementMS = time.Since(start).Milliseconds()

	tickWindow, err := waitForGameTicks(ctx, session, s.Options.TickWindow)
	if err != nil {
		return err
	}
	s.tickWindow = tickWindow

	unitNumbers := make([]int, 0)
	for _, result := range s.instantCopies {
		for _, entity := range result.Created {
			if entity.UnitNumber > 0 {
				unitNumbers = append(unitNumbers, entity.UnitNumber)
			}
		}
	}
	return session.Dispatch("read_tracked_entities", map[string]any{"unit_numbers": unitNumbers}, &s.observed)
}

func (s *BlueprintSmokeScenario) Check(ctx context.Context, session *Session) Result {
	issues := append([]snapshot.Issue{}, placementIssues("ghost", s.ghostResult.Missing)...)
	var expected, created int
	for i, result := range s.instantCopies {
		mode := fmt.Sprintf("instant-copy-%d", i)
		issues = append(issues, placementIssues(mode, result.Missing)...)
		expected += result.ExpectedEntities
		created += result.CreatedCount
	}
	if expected > 0 && created != expected {
		issues = append(issues, issue("blueprint_incomplete_instant_build", "warning", "Blueprint instant build did not create every entity", map[string]any{
			"expected": expected,
			"created":  created,
			"surface":  s.surface,
			"copies":   s.Options.Copies,
		}))
	}

	observedByUnit := map[int]blueprintPlacedEntity{}
	for _, entity := range s.observed.Entities {
		if entity.UnitNumber > 0 {
			observedByUnit[entity.UnitNumber] = entity
		}
	}
	var persisted []map[string]any
	for _, result := range s.instantCopies {
		for _, entity := range result.Created {
			if entity.BlueprintRecipe == "" || entity.UnitNumber == 0 {
				continue
			}
			observed := observedByUnit[entity.UnitNumber]
			if observed.ActualRecipe == entity.BlueprintRecipe {
				persisted = append(persisted, map[string]any{
					"entity":        entity.Name,
					"unit_number":   entity.UnitNumber,
					"recipe":        entity.BlueprintRecipe,
					"actual_recipe": observed.ActualRecipe,
					"recipe_locked": boolPointerValue(observed.RecipeLocked),
				})
			}
		}
	}
	if len(persisted) > 0 {
		severity := "info"
		if s.Document.Features.Parameterized {
			severity = "warning"
		}
		issues = append(issues, issue("blueprint_configured_recipe_persisted", severity, "Blueprint-configured recipe persisted after mod tick logic", map[string]any{
			"persisted_count":     len(persisted),
			"sample_unit_numbers": sampleUnitNumbers(persisted, 20),
			"parameterized":       s.Document.Features.Parameterized,
			"parameter_paths":     s.Document.Features.ParameterPaths,
			"diagnostic_note":     "For signal-controlled machines, this can indicate a player-provided blueprint recipe escaped the mod's recipe reconciliation.",
			"copies":              s.Options.Copies,
			"start_tick":          s.tickWindow.StartTick,
			"end_tick":            s.tickWindow.EndTick,
			"elapsed_ms":          s.tickWindow.ElapsedMS,
		}))
	}

	return Result{
		Issues: issues,
		Observations: map[string]any{
			"blueprint_features": s.Document.Features,
			"options":            s.Options,
			"ghost":              s.ghostResult,
			"instant":            s.instantResult,
			"instant_copies":     s.instantCopies,
			"after_tick":         s.observed,
			"tick_window":        s.tickWindow,
			"placement_ms":       s.placementMS,
		},
	}
}

func boolPointerValue(value *bool) any {
	if value == nil {
		return nil
	}
	return *value
}

func (s *BlueprintSmokeScenario) Cleanup(ctx context.Context, session *Session) error {
	if s.surface == "" {
		return nil
	}
	var deleted struct {
		Deleted bool `json:"deleted"`
	}
	return session.Dispatch("delete_surface", map[string]any{"name": s.surface}, &deleted)
}

func placementIssues(mode string, missing []blueprintMissingEntity) []snapshot.Issue {
	issues := make([]snapshot.Issue, 0, len(missing))
	for _, entity := range missing {
		issues = append(issues, issue("blueprint_entity_placement_failed", "warning", "Blueprint entity could not be placed", map[string]any{
			"mode":   mode,
			"entity": entity.Name,
			"reason": entity.Reason,
		}))
	}
	return issues
}

func cloneStringAny(values map[string]any) map[string]any {
	out := make(map[string]any, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func waitForGameTicks(ctx context.Context, session *Session, ticks int) (tickWindowObservation, error) {
	if ticks <= 0 {
		return tickWindowObservation{}, nil
	}
	var startSummary runtimeSummary
	if err := session.Dispatch("runtime_summary", nil, &startSummary); err != nil {
		return tickWindowObservation{}, err
	}
	start := time.Now()
	target := startSummary.Tick + int64(ticks)
	observation := tickWindowObservation{
		StartTick:      startSummary.Tick,
		RequestedTicks: ticks,
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(tickWindowTimeout(ticks))
	defer timeout.Stop()
	for {
		var summary runtimeSummary
		if err := session.Dispatch("runtime_summary", nil, &summary); err != nil {
			return observation, err
		}
		observation.Polls++
		observation.EndTick = summary.Tick
		observation.ElapsedMS = time.Since(start).Milliseconds()
		if summary.Tick >= target {
			return observation, nil
		}
		select {
		case <-ctx.Done():
			return observation, ctx.Err()
		case <-timeout.C:
			return observation, fmt.Errorf("timed out waiting for %d game ticks; advanced %d ticks", ticks, observation.EndTick-observation.StartTick)
		case <-ticker.C:
		}
	}
}

func tickWindowTimeout(ticks int) time.Duration {
	minimum := 30 * time.Second
	if ticks <= 0 {
		return minimum
	}
	expected := time.Duration(ticks) * time.Second / 60
	timeout := expected * 10
	if timeout < minimum {
		return minimum
	}
	return timeout
}

func copyPosition(index int, spacing int) []float64 {
	columns := 20
	x := 64 + (index%columns)*spacing
	y := (index / columns) * spacing
	return []float64{float64(x), float64(y)}
}

func chunkRadiusForCopies(copies int, spacing int) int {
	if copies <= 1 {
		return 4
	}
	columns := 20
	rows := (copies + columns - 1) / columns
	width := columns * spacing
	if copies < columns {
		width = copies * spacing
	}
	height := rows * spacing
	radius := (width + height) / 32
	if radius < 4 {
		return 4
	}
	return radius + 4
}

func sampleUnitNumbers(values []map[string]any, limit int) []int {
	if len(values) <= limit {
		limit = len(values)
	}
	out := make([]int, 0, limit)
	for i := 0; i < limit; i++ {
		unit, ok := values[i]["unit_number"].(int)
		if ok {
			out = append(out, unit)
		}
	}
	return out
}

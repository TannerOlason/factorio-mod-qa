package qa

import (
	"context"
	"fmt"
	"time"

	"factorio-mod-qa/internal/blueprint"
	"factorio-mod-qa/internal/snapshot"
)

type Scenario interface {
	Name() string
	Setup(context.Context, *Session) error
	Run(context.Context, *Session) error
	Check(context.Context, *Session) Result
	Cleanup(context.Context, *Session) error
}

type Result struct {
	Issues       []snapshot.Issue `json:"issues,omitempty"`
	Observations map[string]any   `json:"observations,omitempty"`
}

type ScenarioRun struct {
	Name       string           `json:"name"`
	IssueCount int              `json:"issue_count"`
	Issues     []snapshot.Issue `json:"issues,omitempty"`
	TracePath  string           `json:"trace_path,omitempty"`
	Error      string           `json:"error,omitempty"`
	DurationMS int64            `json:"duration_ms"`
}

type Trace struct {
	Scenario string       `json:"scenario"`
	Entries  []TraceEntry `json:"entries"`
	nextSeq  int
}

type TraceEntry struct {
	Seq         int    `json:"seq"`
	Phase       string `json:"phase"`
	Action      string `json:"action"`
	Payload     any    `json:"payload,omitempty"`
	Observation any    `json:"observation,omitempty"`
	Error       string `json:"error,omitempty"`
	ElapsedMS   int64  `json:"elapsed_ms,omitempty"`
}

func NewTrace(scenario string) *Trace {
	return &Trace{Scenario: scenario}
}

func (t *Trace) Add(entry TraceEntry) {
	if t == nil {
		return
	}
	t.nextSeq++
	entry.Seq = t.nextSeq
	t.Entries = append(t.Entries, entry)
}

type Runner struct {
	Scenarios []Scenario
	TraceDir  string
}

func (r Runner) Run(ctx context.Context, session *Session) ([]ScenarioRun, error) {
	results := make([]ScenarioRun, 0, len(r.Scenarios))
	for _, scenario := range r.Scenarios {
		result, err := r.runOne(ctx, session, scenario)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func (r Runner) runOne(ctx context.Context, session *Session, scenario Scenario) (ScenarioRun, error) {
	start := time.Now()
	trace := NewTrace(scenario.Name())
	session.Trace = trace
	defer func() {
		session.Trace = nil
	}()

	run := ScenarioRun{Name: scenario.Name()}
	var issues []snapshot.Issue
	if err := scenario.Setup(ctx, session); err != nil {
		run.Error = err.Error()
		issues = append(issues, scenarioErrorIssue(scenario.Name(), err))
	} else if err := scenario.Run(ctx, session); err != nil {
		run.Error = err.Error()
		issues = append(issues, scenarioErrorIssue(scenario.Name(), err))
	} else {
		result := scenario.Check(ctx, session)
		issues = append(issues, result.Issues...)
		if len(result.Observations) > 0 {
			session.Record("check", "observations", nil, result.Observations, nil)
		}
	}

	if err := scenario.Cleanup(ctx, session); err != nil {
		if run.Error == "" {
			run.Error = err.Error()
		}
		issues = append(issues, issue("scenario_cleanup_failed", "warning", fmt.Sprintf("Scenario %s cleanup failed", scenario.Name()), map[string]any{
			"scenario": scenario.Name(),
			"error":    err.Error(),
		}))
	}

	tracePath, err := SaveTrace(r.TraceDir, *trace)
	if err != nil {
		return ScenarioRun{}, err
	}
	run.Issues = issues
	run.IssueCount = len(issues)
	run.TracePath = tracePath
	run.DurationMS = time.Since(start).Milliseconds()
	return run, nil
}

func SelectScenarios(selector string, snap *snapshot.Snapshot, blueprintDoc *blueprint.Document, blueprintOptions BlueprintOptions) ([]Scenario, error) {
	if selector == "" || selector == "all" {
		scenarios := []Scenario{
			NewScriptEventGrowthScenario(),
			NewInventoryAbuseScenario(),
			NewSaveLoadAbuseScenario(),
			NewSurfaceSpawnScenario(),
		}
		if blueprintDoc != nil {
			scenarios = append(scenarios, NewBlueprintSmokeScenario(blueprintDoc, blueprintOptions))
		}
		return scenarios, nil
	}
	if selector == "script-event-growth" {
		return []Scenario{NewScriptEventGrowthScenario()}, nil
	}
	if selector == "inventory-abuse" {
		return []Scenario{NewInventoryAbuseScenario()}, nil
	}
	if selector == "save-load-abuse" {
		return []Scenario{NewSaveLoadAbuseScenario()}, nil
	}
	if selector == "surface-spawn" {
		return []Scenario{NewSurfaceSpawnScenario()}, nil
	}
	if selector == "blueprint-smoke" {
		if blueprintDoc == nil {
			return nil, fmt.Errorf("blueprint-smoke requires --blueprint")
		}
		return []Scenario{NewBlueprintSmokeScenario(blueprintDoc, blueprintOptions)}, nil
	}
	return nil, fmt.Errorf("unknown QA scenario %q", selector)
}

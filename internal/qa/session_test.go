package qa

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"factorio-mod-qa/internal/snapshot"
)

type fakeCommander struct {
	response string
	err      error
	commands []string
}

func (f *fakeCommander) Command(command string) (string, error) {
	f.commands = append(f.commands, command)
	return f.response, f.err
}

func TestDispatchCommandQuotesPayload(t *testing.T) {
	got := DispatchCommand("place_entities_batch", `{"surface":"nauvis"}`)
	for _, want := range []string{
		`remote.call("qa_control_mod", "dispatch"`,
		`"place_entities_batch"`,
		`"{\"surface\":\"nauvis\"}"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("dispatch command missing %q:\n%s", want, got)
		}
	}
}

func TestSessionDispatchDecodesEnvelope(t *testing.T) {
	commander := &fakeCommander{response: `prefix {"ok":true,"result":{"save":"fmqa-test"}}`}
	session := &Session{RCON: commander, Trace: NewTrace("dispatch-test")}

	var result struct {
		Save string `json:"save"`
	}
	if err := session.Dispatch("save", map[string]any{"name": "fmqa-test"}, &result); err != nil {
		t.Fatal(err)
	}
	if result.Save != "fmqa-test" {
		t.Fatalf("save = %q", result.Save)
	}
	if len(session.Trace.Entries) != 1 || session.Trace.Entries[0].Action != "save" {
		t.Fatalf("trace entries = %#v", session.Trace.Entries)
	}
}

func TestSessionDispatchReturnsRemoteError(t *testing.T) {
	session := &Session{RCON: &fakeCommander{response: `{"ok":false,"error":"bad command"}`}}
	if err := session.Dispatch("missing", nil, nil); err == nil || !strings.Contains(err.Error(), "bad command") {
		t.Fatalf("expected remote error, got %v", err)
	}
}

func TestSessionRestartRecordsTrace(t *testing.T) {
	session := &Session{
		Trace: NewTrace("restart-test"),
		RestartFromSave: func(ctx context.Context, save string) error {
			if save != "checkpoint" {
				t.Fatalf("save = %q", save)
			}
			return nil
		},
	}
	if err := session.Restart(context.Background(), "checkpoint"); err != nil {
		t.Fatal(err)
	}
	if len(session.Trace.Entries) != 1 || session.Trace.Entries[0].Action != "restart_from_save" {
		t.Fatalf("trace entries = %#v", session.Trace.Entries)
	}
}

type lifecycleScenario struct {
	calls      []string
	setupErr   error
	runErr     error
	cleanupErr error
}

func (s *lifecycleScenario) Name() string { return "lifecycle" }

func (s *lifecycleScenario) Setup(ctx context.Context, session *Session) error {
	s.calls = append(s.calls, "setup")
	return s.setupErr
}

func (s *lifecycleScenario) Run(ctx context.Context, session *Session) error {
	s.calls = append(s.calls, "run")
	return s.runErr
}

func (s *lifecycleScenario) Check(ctx context.Context, session *Session) Result {
	s.calls = append(s.calls, "check")
	return Result{Issues: []snapshot.Issue{{Code: "test_issue", Severity: "warning", Title: "Test issue"}}}
}

func (s *lifecycleScenario) Cleanup(ctx context.Context, session *Session) error {
	s.calls = append(s.calls, "cleanup")
	return s.cleanupErr
}

func TestRunnerRecordsTraceAndCleanup(t *testing.T) {
	scenario := &lifecycleScenario{}
	results, err := Runner{Scenarios: []Scenario{scenario}, TraceDir: t.TempDir()}.Run(context.Background(), &Session{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(scenario.calls, ",") != "setup,run,check,cleanup" {
		t.Fatalf("calls = %#v", scenario.calls)
	}
	if len(results) != 1 || results[0].IssueCount != 1 || results[0].TracePath == "" {
		t.Fatalf("unexpected scenario results: %#v", results)
	}
	if _, err := os.Stat(results[0].TracePath); err != nil {
		t.Fatalf("expected trace file: %v", err)
	}
}

func TestRunnerCleanupRunsAfterScenarioFailure(t *testing.T) {
	scenario := &lifecycleScenario{runErr: errors.New("runtime failed")}
	results, err := Runner{Scenarios: []Scenario{scenario}, TraceDir: t.TempDir()}.Run(context.Background(), &Session{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(scenario.calls, ",") != "setup,run,cleanup" {
		t.Fatalf("calls = %#v", scenario.calls)
	}
	if results[0].Error != "runtime failed" || results[0].Issues[0].Code != "scenario_failed" {
		t.Fatalf("unexpected failure result: %#v", results[0])
	}
}

func TestScriptStressEntities(t *testing.T) {
	entities := scriptStressEntities("qa-ticking-machine", 5)
	if len(entities) != 5 {
		t.Fatalf("len = %d, wanted 5", len(entities))
	}
	if entities[4]["name"] != "qa-ticking-machine" {
		t.Fatalf("unexpected entity name: %#v", entities[4])
	}
}

func TestSelectScenariosIncludesInventoryAbuse(t *testing.T) {
	scenarios, err := SelectScenarios("all", nil)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(scenarios))
	for _, scenario := range scenarios {
		names = append(names, scenario.Name())
	}
	if strings.Join(names, ",") != "script-event-growth,inventory-abuse,save-load-abuse" {
		t.Fatalf("scenario names = %#v", names)
	}

	scenarios, err = SelectScenarios("inventory-abuse", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(scenarios) != 1 || scenarios[0].Name() != "inventory-abuse" {
		t.Fatalf("inventory-abuse selector = %#v", scenarios)
	}

	scenarios, err = SelectScenarios("save-load-abuse", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(scenarios) != 1 || scenarios[0].Name() != "save-load-abuse" {
		t.Fatalf("save-load-abuse selector = %#v", scenarios)
	}
}

func TestExpectedMineableProducts(t *testing.T) {
	snap := &snapshot.Snapshot{
		Entities: map[string]map[string]any{
			"steel-chest": {
				"mineable_properties": map[string]any{
					"products": []any{
						map[string]any{"name": "steel-chest", "amount": float64(1)},
					},
				},
			},
		},
	}
	expected := expectedMineableProducts(snap, "steel-chest")
	if expected["steel-chest"] != 1 {
		t.Fatalf("expected mineable products = %#v", expected)
	}
}

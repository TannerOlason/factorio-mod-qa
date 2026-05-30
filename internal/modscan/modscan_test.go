package modscan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanDetectsTickStorageLoopAndEstimate(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "control.lua"), `
local machine = require("scripts.machine")
script.on_event(defines.events.on_tick, function(event)
  machine.tick(event.tick)
end)
`)
	writeFile(t, filepath.Join(root, "scripts", "machine.lua"), `
local CHECK_INTERVAL = 60 * 5
local machine = {}
function machine.tick(tick)
  if tick % CHECK_INTERVAL ~= 0 then return end
  for un, entry in pairs(storage.fp_machines) do
    entry.count = (entry.count or 0) + 1
  end
end
return machine
`)

	report, err := Scan(Options{ModDir: root, MegaBaseEntities: 10000})
	if err != nil {
		t.Fatal(err)
	}
	if report.LuaFilesScanned != 2 {
		t.Fatalf("lua files = %d", report.LuaFilesScanned)
	}
	if len(report.OnTickHandlers) != 1 {
		t.Fatalf("on_tick handlers = %#v", report.OnTickHandlers)
	}
	if len(report.StorageLoops) != 1 {
		t.Fatalf("storage loops = %#v", report.StorageLoops)
	}
	loop := report.StorageLoops[0]
	if loop.Storage != "storage.fp_machines" || loop.IntervalTicks != 300 {
		t.Fatalf("loop = %#v", loop)
	}
	if loop.EstimatedIterationsPerSec != 2000 {
		t.Fatalf("estimate = %d", loop.EstimatedIterationsPerSec)
	}
	if loop.Risk != "low" {
		t.Fatalf("risk = %q", loop.Risk)
	}
}

func TestScanSkipsNodeModules(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "node_modules", "noise.lua"), `
script.on_event(defines.events.on_tick, function() end)
`)
	report, err := Scan(Options{ModDir: root})
	if err != nil {
		t.Fatal(err)
	}
	if report.LuaFilesScanned != 0 || len(report.OnTickHandlers) != 0 {
		t.Fatalf("unexpected node_modules scan: %#v", report)
	}
}

func TestEvalIntExpression(t *testing.T) {
	got, ok := evalIntExpression("60 * 60 * 5")
	if !ok || got != 18000 {
		t.Fatalf("got %d, ok=%v", got, ok)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

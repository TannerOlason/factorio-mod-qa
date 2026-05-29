package runner

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecodeSnapshotResponseExtractsJSON(t *testing.T) {
	snap, err := decodeSnapshotResponse(`Notice: {"schema_version":1,"active_mods":{"base":"2.0.73"}}`)
	if err != nil {
		t.Fatal(err)
	}
	if snap.SchemaVersion != 1 {
		t.Fatalf("schema version = %d, wanted 1", snap.SchemaVersion)
	}
	if snap.ActiveMods["base"] != "2.0.73" {
		t.Fatalf("base mod = %q", snap.ActiveMods["base"])
	}
}

func TestLuaStringQuotesPayload(t *testing.T) {
	got := luaString(`{"surface":"nauvis"}`)
	want := `"{\"surface\":\"nauvis\"}"`
	if got != want {
		t.Fatalf("luaString() = %s, wanted %s", got, want)
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

func TestDoctorReportsResolvedFactorioAndControlMod(t *testing.T) {
	root := t.TempDir()
	factorioBin := filepath.Join(root, "factorio")
	if err := os.WriteFile(factorioBin, []byte("#!/bin/sh\necho 'Version: 2.0.76 (build 84451, linux64, full, space-age)'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	controlMod := filepath.Join(root, "qa_control_mod")
	if err := os.MkdirAll(controlMod, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(controlMod, "info.json"), []byte(`{"name":"qa_control_mod"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := Doctor(DoctorOptions{FactorioBin: factorioBin, ControlModPath: controlMod}, &out); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"factorio_bin=" + factorioBin,
		"factorio_version=Version: 2.0.76",
		"qa_control_mod=" + controlMod,
		"status=ok",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("doctor output missing %q:\n%s", want, text)
		}
	}
}

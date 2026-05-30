package blueprint

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecodeStringReadsBlueprintExchangeString(t *testing.T) {
	input := encodeExchangeString(t, `{
  "blueprint": {
    "label": "smoke",
    "entities": [
      {
        "entity_number": 1,
        "name": "stone-furnace",
        "position": {"x": 0, "y": 0}
      }
    ],
    "tiles": [
      {
        "name": "stone-path",
        "position": {"x": 1, "y": 1}
      }
    ],
    "item": "blueprint",
    "version": 281479275675648
  }
}`)

	doc, err := DecodeString(input)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Kind != "blueprint" || doc.Label != "smoke" || doc.EntityCount != 1 || doc.TileCount != 1 {
		t.Fatalf("unexpected summary: %#v", doc)
	}
	if doc.Raw == nil {
		t.Fatal("expected raw blueprint JSON")
	}
	if doc.Features.EntityNames[0] != "stone-furnace" {
		t.Fatalf("entity names = %#v", doc.Features.EntityNames)
	}
}

func TestDecodeStringReadsBlueprintBook(t *testing.T) {
	doc, err := DecodeString(`{
  "blueprint_book": {
    "label": "book",
    "blueprints": [
      {
        "index": 0,
        "blueprint": {
          "entities": [{"entity_number": 1, "name": "transport-belt"}]
        }
      },
      {
        "index": 1,
        "blueprint": {
          "entities": [{"entity_number": 1, "name": "inserter"}]
        }
      }
    ]
  }
}`)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Kind != "blueprint_book" || doc.Label != "book" || doc.BlueprintCount != 2 || doc.EntityCount != 2 {
		t.Fatalf("unexpected book summary: %#v", doc)
	}
}

func TestDecodeInputReadsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blueprint.txt")
	if err := os.WriteFile(path, []byte(`{"blueprint":{"label":"file"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := DecodeInput(path)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Label != "file" {
		t.Fatalf("label = %q", doc.Label)
	}
}

func TestDecodeStringDetectsParameterizedConfiguredBlueprint(t *testing.T) {
	doc, err := DecodeString(`{
  "blueprint": {
    "label": "parameterized",
    "parameters": [
      {"id": "parameter-0", "name": "Target recipe"}
    ],
    "entities": [
      {
        "entity_number": 1,
        "name": "assembling-machine-2",
        "position": {"x": 0, "y": 0},
        "recipe": "iron-gear-wheel",
        "control_behavior": {
          "circuit_condition": {
            "first_signal": {"type": "virtual", "name": "signal-parameter-0"}
          }
        },
        "items": {"speed-module": 1}
      }
    ]
  }
}`)
	if err != nil {
		t.Fatal(err)
	}
	if !doc.Features.Parameterized {
		t.Fatalf("expected parameterized features: %#v", doc.Features)
	}
	if doc.Features.ConfiguredEntityCount != 1 {
		t.Fatalf("configured entity count = %d", doc.Features.ConfiguredEntityCount)
	}
	if strings.Join(doc.Features.ConfiguredEntityFields, ",") != "control_behavior,items,recipe" {
		t.Fatalf("configured fields = %#v", doc.Features.ConfiguredEntityFields)
	}
	if strings.Join(doc.Features.Recipes, ",") != "iron-gear-wheel" {
		t.Fatalf("recipes = %#v", doc.Features.Recipes)
	}
}

func TestDecodeStringRejectsInvalidInput(t *testing.T) {
	if _, err := DecodeString("bad"); err == nil {
		t.Fatal("expected invalid blueprint exchange error")
	}
	if _, err := DecodeString(`{"not_blueprint":true}`); err == nil {
		t.Fatal("expected missing blueprint root error")
	}
}

func encodeExchangeString(t *testing.T, jsonText string) string {
	t.Helper()
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write([]byte(jsonText)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return "0" + base64.StdEncoding.EncodeToString(compressed.Bytes())
}

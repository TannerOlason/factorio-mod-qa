package blueprint

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

type Document struct {
	Kind           string         `json:"kind"`
	Label          string         `json:"label,omitempty"`
	EntityCount    int            `json:"entity_count"`
	TileCount      int            `json:"tile_count"`
	BlueprintCount int            `json:"blueprint_count,omitempty"`
	Features       Features       `json:"features"`
	Raw            map[string]any `json:"raw,omitempty"`
}

type Features struct {
	Parameterized          bool     `json:"parameterized"`
	ParameterPaths         []string `json:"parameter_paths,omitempty"`
	ConfiguredEntityCount  int      `json:"configured_entity_count,omitempty"`
	ConfiguredEntityFields []string `json:"configured_entity_fields,omitempty"`
	EntityNames            []string `json:"entity_names,omitempty"`
	Recipes                []string `json:"recipes,omitempty"`
}

func DecodeInput(value string) (*Document, error) {
	text, err := readInput(value)
	if err != nil {
		return nil, err
	}
	return DecodeString(text)
}

func DecodeString(value string) (*Document, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errors.New("blueprint input is empty")
	}
	var data []byte
	if strings.HasPrefix(value, "{") {
		data = []byte(value)
	} else {
		decoded, err := decodeExchangeString(value)
		if err != nil {
			return nil, err
		}
		data = decoded
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("blueprint JSON decode failed: %w", err)
	}
	doc := summarize(raw)
	if doc.Kind == "" {
		return nil, errors.New("blueprint JSON must contain blueprint or blueprint_book")
	}
	doc.Features = analyzeFeatures(raw)
	doc.Raw = raw
	return &doc, nil
}

func decodeExchangeString(value string) ([]byte, error) {
	if !strings.HasPrefix(value, "0") {
		return nil, errors.New("blueprint exchange string must start with version byte 0")
	}
	payload := value[1:]
	compressed, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("blueprint base64 decode failed: %w", err)
	}
	reader, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("blueprint zlib decode failed: %w", err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("blueprint zlib read failed: %w", err)
	}
	return data, nil
}

func readInput(value string) (string, error) {
	if data, err := os.ReadFile(value); err == nil {
		return string(data), nil
	}
	return value, nil
}

func summarize(raw map[string]any) Document {
	if blueprint, ok := object(raw["blueprint"]); ok {
		return summarizeBlueprint(blueprint)
	}
	if book, ok := object(raw["blueprint_book"]); ok {
		doc := Document{
			Kind:  "blueprint_book",
			Label: stringValue(book["label"]),
		}
		for _, entry := range list(book["blueprints"]) {
			entryObj, ok := object(entry)
			if !ok {
				continue
			}
			if blueprint, ok := object(entryObj["blueprint"]); ok {
				child := summarizeBlueprint(blueprint)
				doc.BlueprintCount++
				doc.EntityCount += child.EntityCount
				doc.TileCount += child.TileCount
			}
		}
		return doc
	}
	return Document{}
}

func summarizeBlueprint(blueprint map[string]any) Document {
	return Document{
		Kind:        "blueprint",
		Label:       stringValue(blueprint["label"]),
		EntityCount: len(list(blueprint["entities"])),
		TileCount:   len(list(blueprint["tiles"])),
	}
}

func analyzeFeatures(raw map[string]any) Features {
	features := featureCollector{
		parameterPaths: map[string]bool{},
		entityFields:   map[string]bool{},
		entityNames:    map[string]bool{},
		recipes:        map[string]bool{},
	}
	features.walk("", raw)
	for _, blueprint := range blueprints(raw) {
		features.inspectBlueprint(blueprint)
	}
	return Features{
		Parameterized:          len(features.parameterPaths) > 0,
		ParameterPaths:         sortedSet(features.parameterPaths),
		ConfiguredEntityCount:  features.configuredEntityCount,
		ConfiguredEntityFields: sortedSet(features.entityFields),
		EntityNames:            sortedSet(features.entityNames),
		Recipes:                sortedSet(features.recipes),
	}
}

type featureCollector struct {
	parameterPaths        map[string]bool
	entityFields          map[string]bool
	entityNames           map[string]bool
	recipes               map[string]bool
	configuredEntityCount int
}

func (c *featureCollector) walk(path string, value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			if strings.Contains(strings.ToLower(key), "parameter") {
				c.parameterPaths[childPath] = true
			}
			c.walk(childPath, child)
		}
	case []any:
		for i, child := range typed {
			c.walk(fmt.Sprintf("%s[%d]", path, i), child)
		}
	case string:
		lower := strings.ToLower(typed)
		if strings.Contains(lower, "signal-parameter-") || strings.HasPrefix(lower, "parameter-") {
			c.parameterPaths[path] = true
		}
	}
}

func (c *featureCollector) inspectBlueprint(blueprint map[string]any) {
	for _, entityValue := range list(blueprint["entities"]) {
		entity, ok := object(entityValue)
		if !ok {
			continue
		}
		if name := stringValue(entity["name"]); name != "" {
			c.entityNames[name] = true
		}
		if recipe := stringValue(entity["recipe"]); recipe != "" {
			c.recipes[recipe] = true
		}
		configured := false
		for _, field := range configuredEntityFields {
			if _, ok := entity[field]; ok {
				c.entityFields[field] = true
				configured = true
			}
		}
		if configured {
			c.configuredEntityCount++
		}
	}
}

var configuredEntityFields = []string{
	"ammo_inventory",
	"bar",
	"connections",
	"control_behavior",
	"filters",
	"infinity_settings",
	"inventory",
	"items",
	"parameters",
	"recipe",
	"request_filters",
	"station",
	"tags",
	"trunk_inventory",
}

func blueprints(raw map[string]any) []map[string]any {
	if blueprint, ok := object(raw["blueprint"]); ok {
		return []map[string]any{blueprint}
	}
	book, ok := object(raw["blueprint_book"])
	if !ok {
		return nil
	}
	var out []map[string]any
	for _, entry := range list(book["blueprints"]) {
		entryObj, ok := object(entry)
		if !ok {
			continue
		}
		if blueprint, ok := object(entryObj["blueprint"]); ok {
			out = append(out, blueprint)
		}
	}
	return out
}

func sortedSet(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func object(value any) (map[string]any, bool) {
	obj, ok := value.(map[string]any)
	return obj, ok
}

func list(value any) []any {
	items, _ := value.([]any)
	return items
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

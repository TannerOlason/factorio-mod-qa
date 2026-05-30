package modscan

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const defaultMegaBaseEntities = 10000

type Options struct {
	ModDir           string
	MegaBaseEntities int
}

type Report struct {
	ModDir            string              `json:"mod_dir"`
	MegaBaseEntities  int                 `json:"mega_base_entities"`
	LuaFilesScanned   int                 `json:"lua_files_scanned"`
	OnTickHandlers    []Finding           `json:"on_tick_handlers,omitempty"`
	TickCalls         []Finding           `json:"tick_calls,omitempty"`
	TickFunctions     []Finding           `json:"tick_functions,omitempty"`
	StorageLoops      []StorageLoop       `json:"storage_loops,omitempty"`
	HotLoopEstimate   Estimate            `json:"hot_loop_estimate"`
	HighRiskStorage   []StorageLoop       `json:"high_risk_storage,omitempty"`
	Notes             []string            `json:"notes,omitempty"`
	IntervalConstants map[string]Interval `json:"interval_constants,omitempty"`
	constantsByFile   map[string]map[string]Interval
	tickRangesByFile  map[string][]lineRange
}

type Finding struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

type Interval struct {
	Name  string `json:"name"`
	Ticks int    `json:"ticks"`
	File  string `json:"file,omitempty"`
	Line  int    `json:"line,omitempty"`
}

type StorageLoop struct {
	File                      string   `json:"file"`
	Line                      int      `json:"line"`
	Storage                   string   `json:"storage"`
	Text                      string   `json:"text"`
	IntervalTicks             int      `json:"interval_ticks"`
	IntervalSource            string   `json:"interval_source,omitempty"`
	Scope                     string   `json:"scope"`
	EstimatedIterationsPerSec int      `json:"estimated_iterations_per_sec"`
	Risk                      string   `json:"risk"`
	Reasons                   []string `json:"reasons,omitempty"`
}

type Estimate struct {
	EntityBackedLoops         int `json:"entity_backed_loops"`
	EstimatedIterationsPerSec int `json:"estimated_iterations_per_sec"`
}

type luaFile struct {
	path  string
	lines []string
}

type lineRange struct {
	start int
	end   int
	name  string
}

var (
	onTickRE         = regexp.MustCompile(`script\.on_event\s*\(\s*defines\.events\.on_tick`)
	onNthTickRE      = regexp.MustCompile(`script\.on_nth_tick\s*\(`)
	tickCallRE       = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z0-9_]*tick[A-Za-z0-9_]*)\s*\(`)
	functionRE       = regexp.MustCompile(`^\s*function\s+([A-Za-z_][A-Za-z0-9_\.]*)\s*\(`)
	storageLoopRE    = regexp.MustCompile(`\bfor\s+.+\s+in\s+(?:pairs|ipairs)\s*\(\s*(storage(?:\.[A-Za-z0-9_]+)+)`)
	intervalAssignRE = regexp.MustCompile(`^\s*(?:local\s+)?([A-Z][A-Z0-9_]*(?:INTERVAL|TICKS)[A-Z0-9_]*)\s*=\s*([0-9][0-9\s\*\+/\-\(\)]*)`)
	intervalGuardRE  = regexp.MustCompile(`(?:tick|current_tick)\s*%\s*([A-Z][A-Z0-9_]*|[0-9]+)\s*[~!<>=]=?\s*0`)
)

func Scan(opts Options) (*Report, error) {
	if opts.ModDir == "" {
		return nil, fmt.Errorf("--mod-dir is required")
	}
	if opts.MegaBaseEntities <= 0 {
		opts.MegaBaseEntities = defaultMegaBaseEntities
	}
	root, err := filepath.EvalSymlinks(opts.ModDir)
	if err != nil {
		return nil, err
	}
	files, err := readLuaFiles(root)
	if err != nil {
		return nil, err
	}

	report := &Report{
		ModDir:            root,
		MegaBaseEntities:  opts.MegaBaseEntities,
		LuaFilesScanned:   len(files),
		IntervalConstants: map[string]Interval{},
		constantsByFile:   map[string]map[string]Interval{},
		tickRangesByFile:  map[string][]lineRange{},
	}
	for _, file := range files {
		report.scanConstants(file)
	}
	for _, file := range files {
		report.tickRangesByFile[file.path] = tickRanges(file)
	}
	for _, file := range files {
		report.scanFile(file)
	}
	report.finalize()
	return report, nil
}

func readLuaFiles(root string) ([]luaFile, error) {
	var files []luaFile
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if shouldSkipDir(entry.Name()) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".lua" {
			return nil
		}
		lines, err := readLines(path)
		if err != nil {
			return err
		}
		files = append(files, luaFile{path: path, lines: lines})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	return files, nil
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", ".factorio-test", "node_modules", "tests", ".vscode", ".idea":
		return true
	default:
		return false
	}
}

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func (r *Report) scanConstants(file luaFile) {
	rel := r.rel(file.path)
	for i, line := range file.lines {
		match := intervalAssignRE.FindStringSubmatch(stripLuaComment(line))
		if len(match) == 0 {
			continue
		}
		ticks, ok := evalIntExpression(match[2])
		if !ok || ticks <= 0 {
			continue
		}
		interval := Interval{Name: match[1], Ticks: ticks, File: rel, Line: i + 1}
		r.IntervalConstants[rel+":"+match[1]] = interval
		if r.constantsByFile[file.path] == nil {
			r.constantsByFile[file.path] = map[string]Interval{}
		}
		r.constantsByFile[file.path][match[1]] = interval
	}
}

func (r *Report) scanFile(file luaFile) {
	rel := r.rel(file.path)
	for i, line := range file.lines {
		trimmed := strings.TrimSpace(line)
		if onTickRE.MatchString(line) || onNthTickRE.MatchString(line) {
			r.OnTickHandlers = append(r.OnTickHandlers, Finding{File: rel, Line: i + 1, Text: trimmed})
		}
		if match := functionRE.FindStringSubmatch(line); len(match) > 0 && isTickName(match[1]) {
			r.TickFunctions = append(r.TickFunctions, Finding{File: rel, Line: i + 1, Text: trimmed})
		}
		for _, match := range tickCallRE.FindAllStringSubmatch(line, -1) {
			if !isTickName(match[2]) {
				continue
			}
			text := strings.TrimSpace(match[0])
			r.TickCalls = append(r.TickCalls, Finding{File: rel, Line: i + 1, Text: text})
		}
		if match := storageLoopRE.FindStringSubmatch(line); len(match) > 0 {
			if _, ok := r.tickContext(file.path, i+1); !ok {
				continue
			}
			loop := StorageLoop{
				File:    rel,
				Line:    i + 1,
				Storage: match[1],
				Text:    trimmed,
			}
			r.classifyLoop(&loop, file.lines, i)
			r.StorageLoops = append(r.StorageLoops, loop)
		}
	}
}

func (r *Report) classifyLoop(loop *StorageLoop, lines []string, index int) {
	interval, source := r.nearestInterval(loop.File, lines, index)
	if interval <= 0 {
		interval = 1
		source = "none"
	}
	loop.IntervalTicks = interval
	loop.IntervalSource = source
	loop.Scope = storageScope(loop.Storage)
	if loop.Scope == "entity" {
		loop.EstimatedIterationsPerSec = (60 * r.MegaBaseEntities) / interval
	} else {
		loop.EstimatedIterationsPerSec = 0
	}
	loop.Risk, loop.Reasons = riskForLoop(*loop)
}

func (r *Report) nearestInterval(file string, lines []string, index int) (int, string) {
	start := index - 40
	if start < 0 {
		start = 0
	}
	for i := index; i >= start; i-- {
		match := intervalGuardRE.FindStringSubmatch(lines[i])
		if len(match) == 0 {
			continue
		}
		token := match[1]
		if ticks, err := strconv.Atoi(token); err == nil {
			return ticks, fmt.Sprintf("line %d literal %d", i+1, ticks)
		}
		abs := filepath.Join(r.ModDir, filepath.FromSlash(file))
		if interval, ok := r.constantsByFile[abs][token]; ok {
			return interval.Ticks, fmt.Sprintf("%s=%d", token, interval.Ticks)
		}
		return 1, "unknown guard " + token
	}
	return 1, "none"
}

func stripLuaComment(line string) string {
	if index := strings.Index(line, "--"); index >= 0 {
		return line[:index]
	}
	return line
}

func (r *Report) tickContext(path string, line int) (lineRange, bool) {
	for _, candidate := range r.tickRangesByFile[path] {
		if line >= candidate.start && line <= candidate.end {
			return candidate, true
		}
	}
	return lineRange{}, false
}

func (r *Report) finalize() {
	sort.Slice(r.StorageLoops, func(i, j int) bool {
		if r.StorageLoops[i].Risk == r.StorageLoops[j].Risk {
			return r.StorageLoops[i].EstimatedIterationsPerSec > r.StorageLoops[j].EstimatedIterationsPerSec
		}
		return riskRank(r.StorageLoops[i].Risk) > riskRank(r.StorageLoops[j].Risk)
	})
	for _, loop := range r.StorageLoops {
		if loop.Scope != "entity" {
			continue
		}
		r.HotLoopEstimate.EntityBackedLoops++
		r.HotLoopEstimate.EstimatedIterationsPerSec += loop.EstimatedIterationsPerSec
		if loop.Risk == "high" || loop.Risk == "medium" {
			r.HighRiskStorage = append(r.HighRiskStorage, loop)
		}
	}
	r.Notes = []string{
		"Estimates are static heuristics: they assume the selected mega-base entity count is present in each entity-backed storage table.",
		"Loops over players, forces, GUI state, routes, surfaces, or pending deliveries are reported but excluded from entity-count iteration totals.",
		"Any entity-backed loop with no interval guard is modeled as every tick and should be verified manually.",
	}
}

func (r *Report) rel(path string) string {
	rel, err := filepath.Rel(r.ModDir, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}

func tickRanges(file luaFile) []lineRange {
	var ranges []lineRange
	var current *lineRange
	for i, line := range file.lines {
		match := functionRE.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		if current != nil {
			current.end = i
			ranges = append(ranges, *current)
			current = nil
		}
		if isTickName(match[1]) {
			current = &lineRange{start: i + 1, end: len(file.lines), name: match[1]}
		}
	}
	if current != nil {
		ranges = append(ranges, *current)
	}
	return ranges
}

func isTickName(name string) bool {
	parts := strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return r == '.' || r == ':'
	})
	if len(parts) == 0 {
		return false
	}
	last := parts[len(parts)-1]
	return last == "tick" ||
		strings.HasPrefix(last, "tick_") ||
		strings.HasSuffix(last, "_tick") ||
		strings.Contains(last, "_tick_")
}

func storageScope(storage string) string {
	lower := strings.ToLower(storage)
	switch {
	case strings.Contains(lower, "gui") || strings.Contains(lower, "player"):
		return "player"
	case strings.Contains(lower, "force") || strings.Contains(lower, "stock") || strings.Contains(lower, "orders"):
		return "force"
	case strings.Contains(lower, "route") || strings.Contains(lower, "trip") || strings.Contains(lower, "platform") || strings.Contains(lower, "surface"):
		return "world"
	case strings.Contains(lower, "pending") || strings.Contains(lower, "delivery"):
		return "queue"
	default:
		return "entity"
	}
}

func riskForLoop(loop StorageLoop) (string, []string) {
	var reasons []string
	if loop.Scope == "entity" {
		reasons = append(reasons, "entity-backed storage loop")
	}
	if loop.IntervalTicks <= 1 {
		reasons = append(reasons, "no interval guard detected")
	}
	if loop.EstimatedIterationsPerSec >= 100000 {
		reasons = append(reasons, "mega-base estimate is at least 100k loop iterations/sec")
		return "high", reasons
	}
	if loop.EstimatedIterationsPerSec >= 10000 {
		reasons = append(reasons, "mega-base estimate is at least 10k loop iterations/sec")
		return "medium", reasons
	}
	if loop.Scope == "entity" {
		return "low", reasons
	}
	return "info", reasons
}

func riskRank(risk string) int {
	switch risk {
	case "high":
		return 4
	case "medium":
		return 3
	case "low":
		return 2
	default:
		return 1
	}
}

func evalIntExpression(value string) (int, bool) {
	parser := expressionParser{text: strings.ReplaceAll(value, " ", "")}
	result, ok := parser.parseExpression()
	if !ok || parser.pos != len(parser.text) {
		return 0, false
	}
	return result, true
}

type expressionParser struct {
	text string
	pos  int
}

func (p *expressionParser) parseExpression() (int, bool) {
	value, ok := p.parseTerm()
	if !ok {
		return 0, false
	}
	for p.pos < len(p.text) {
		op := p.text[p.pos]
		if op != '+' && op != '-' {
			break
		}
		p.pos++
		right, ok := p.parseTerm()
		if !ok {
			return 0, false
		}
		if op == '+' {
			value += right
		} else {
			value -= right
		}
	}
	return value, true
}

func (p *expressionParser) parseTerm() (int, bool) {
	value, ok := p.parseFactor()
	if !ok {
		return 0, false
	}
	for p.pos < len(p.text) {
		op := p.text[p.pos]
		if op != '*' && op != '/' {
			break
		}
		p.pos++
		right, ok := p.parseFactor()
		if !ok || right == 0 {
			return 0, false
		}
		if op == '*' {
			value *= right
		} else {
			value /= right
		}
	}
	return value, true
}

func (p *expressionParser) parseFactor() (int, bool) {
	if p.pos >= len(p.text) {
		return 0, false
	}
	if p.text[p.pos] == '(' {
		p.pos++
		value, ok := p.parseExpression()
		if !ok || p.pos >= len(p.text) || p.text[p.pos] != ')' {
			return 0, false
		}
		p.pos++
		return value, true
	}
	start := p.pos
	for p.pos < len(p.text) && p.text[p.pos] >= '0' && p.text[p.pos] <= '9' {
		p.pos++
	}
	if start == p.pos {
		return 0, false
	}
	value, err := strconv.Atoi(p.text[start:p.pos])
	return value, err == nil
}

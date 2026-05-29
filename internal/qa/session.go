package qa

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"factorio-mod-qa/internal/factorio"
	"factorio-mod-qa/internal/snapshot"
)

type Commander interface {
	Command(command string) (string, error)
}

type Session struct {
	RCON      Commander
	Process   *factorio.Process
	Snapshot  *snapshot.Snapshot
	RunID     string
	Trace     *Trace
	Artifacts map[string]string

	RestartFromSave func(context.Context, string) error
}

type dispatchEnvelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

func (s *Session) Dispatch(command string, payload any, out any) error {
	if s == nil || s.RCON == nil {
		return errors.New("qa session has no RCON client")
	}
	if payload == nil {
		payload = map[string]any{}
	}
	payloadData, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	rconCommand := DispatchCommand(command, string(payloadData))
	start := time.Now()
	raw, err := s.RCON.Command(rconCommand)
	elapsed := time.Since(start)
	if err != nil {
		s.Record("dispatch", command, payload, nil, err)
		return err
	}
	var envelope dispatchEnvelope
	if err := json.Unmarshal([]byte(extractJSONObject(raw)), &envelope); err != nil {
		s.Record("dispatch", command, payload, raw, err)
		return err
	}
	if !envelope.OK {
		if envelope.Error == "" {
			envelope.Error = "qa_control_mod dispatch failed"
		}
		err := errors.New(envelope.Error)
		s.Record("dispatch", command, payload, envelope, err)
		return err
	}
	if out != nil && len(envelope.Result) > 0 {
		if err := json.Unmarshal(envelope.Result, out); err != nil {
			s.Record("dispatch", command, payload, envelope, err)
			return err
		}
	}
	s.RecordElapsed("dispatch", command, payload, envelope.Result, "", elapsed)
	return nil
}

func (s *Session) Restart(ctx context.Context, saveName string) error {
	if s == nil || s.RestartFromSave == nil {
		return errors.New("qa session cannot restart from save")
	}
	start := time.Now()
	err := s.RestartFromSave(ctx, saveName)
	if err != nil {
		s.RecordElapsed("restart", "restart_from_save", map[string]any{"save": saveName}, nil, err.Error(), time.Since(start))
		return err
	}
	s.RecordElapsed("restart", "restart_from_save", map[string]any{"save": saveName}, map[string]any{"ok": true}, "", time.Since(start))
	return nil
}

func DispatchCommand(command string, payloadJSON string) string {
	return `/silent-command rcon.print(remote.call("qa_control_mod", "dispatch", ` + luaString(command) + `, ` + luaString(payloadJSON) + `))`
}

func (s *Session) Record(phase string, action string, payload any, observation any, err error) {
	errText := ""
	if err != nil {
		errText = err.Error()
	}
	s.RecordElapsed(phase, action, payload, observation, errText, 0)
}

func (s *Session) RecordElapsed(phase string, action string, payload any, observation any, errText string, elapsed time.Duration) {
	if s == nil || s.Trace == nil {
		return
	}
	s.Trace.Add(TraceEntry{
		Phase:       phase,
		Action:      action,
		Payload:     payload,
		Observation: observation,
		Error:       errText,
		ElapsedMS:   elapsed.Milliseconds(),
	})
}

func SaveTrace(dir string, trace Trace) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "trace-"+sanitizeName(trace.Scenario)+".json")
	data, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, append(data, '\n'), 0o644)
}

func extractJSONObject(raw string) string {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end < start {
		return ""
	}
	return raw[start : end+1]
}

func luaString(value string) string {
	return strconv.Quote(value)
}

func sanitizeName(value string) string {
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else if r == ' ' || r == '/' {
			b.WriteByte('-')
		}
	}
	if b.Len() == 0 {
		return "scenario"
	}
	return b.String()
}

func issue(code string, severity string, title string, details map[string]any) snapshot.Issue {
	return snapshot.Issue{
		Code:     code,
		Severity: severity,
		Title:    title,
		Details:  details,
	}
}

func scenarioErrorIssue(name string, err error) snapshot.Issue {
	return issue("scenario_failed", "warning", fmt.Sprintf("Scenario %s failed", name), map[string]any{
		"scenario": name,
		"error":    err.Error(),
	})
}

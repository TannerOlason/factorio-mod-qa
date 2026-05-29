package reports

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"factorio-mod-qa/internal/snapshot"
)

type Summary struct {
	RunID        string            `json:"run_id"`
	SnapshotPath string            `json:"snapshot_path,omitempty"`
	IssueCount   int               `json:"issue_count"`
	Issues       []snapshot.Issue  `json:"issues"`
	Artifacts    map[string]string `json:"artifacts,omitempty"`
}

func Write(dir string, summary Summary) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "summary.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "summary.md"), []byte(markdown(summary)), 0o644)
}

func markdown(summary Summary) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Factorio Mod QA Summary\n\n")
	if summary.RunID != "" {
		fmt.Fprintf(&b, "- Run ID: `%s`\n", summary.RunID)
	}
	if summary.SnapshotPath != "" {
		fmt.Fprintf(&b, "- Snapshot: `%s`\n", summary.SnapshotPath)
	}
	fmt.Fprintf(&b, "- Issues: %d\n\n", summary.IssueCount)
	if len(summary.Issues) == 0 {
		b.WriteString("No static validation issues were found.\n")
		return b.String()
	}
	b.WriteString("## Issues\n\n")
	for _, issue := range summary.Issues {
		fmt.Fprintf(&b, "### %s: %s\n\n", issue.Severity, issue.Title)
		fmt.Fprintf(&b, "- Code: `%s`\n", issue.Code)
		if len(issue.Details) > 0 {
			keys := make([]string, 0, len(issue.Details))
			for key := range issue.Details {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				fmt.Fprintf(&b, "- %s: `%v`\n", key, issue.Details[key])
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

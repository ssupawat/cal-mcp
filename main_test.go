package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestListCalendarsMock(t *testing.T) {
	orig := runAppleScript
	defer func() { runAppleScript = orig }()

	runAppleScript = func(script string) (string, error) {
		if !strings.Contains(script, "calendars") {
			t.Errorf("expected script to contain calendars, got: %s", script)
		}
		return "Work\nHome\nPlay", nil
	}

	result, err := listCalendars()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var calendars []string
	if err := json.Unmarshal([]byte(result), &calendars); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	expected := []string{"Work", "Home", "Play"}
	if len(calendars) != len(expected) {
		t.Fatalf("expected %d calendars, got %d", len(expected), len(calendars))
	}
	for i, name := range calendars {
		if name != expected[i] {
			t.Errorf("expected calendar %d to be %q, got %q", i, expected[i], name)
		}
	}
}

func TestUpdateEventMock(t *testing.T) {
	orig := runAppleScript
	defer func() { runAppleScript = orig }()

	_, err := updateEvent(map[string]interface{}{})
	if err == nil || !strings.Contains(err.Error(), "uid is required") {
		t.Errorf("expected error for missing uid, got: %v", err)
	}

	runAppleScript = func(script string) (string, error) {
		expectedContents := []string{
			"set targetUid to \"12345\"",
			"set summary of e to \"New Title\"",
			"set startD to (my buildDate(2026, 6, 2, 10, 0, 0))",
			"set endD to (my buildDate(2026, 6, 2, 11, 0, 0))",
			"move e to end of events of calendar \"Work\"",
		}
		for _, content := range expectedContents {
			if !strings.Contains(script, content) {
				t.Errorf("expected script to contain: %q, but got: %s", content, script)
			}
		}
		// Return mocked applescript return string (found | calendar | summary | start | end)
		return "true|Work|New Title|2026-06-02T10:00:00|2026-06-02T11:00:00", nil
	}

	args := map[string]interface{}{
		"uid":      "12345",
		"title":    "New Title",
		"start":    "2026-06-02T10:00:00",
		"end":      "2026-06-02T11:00:00",
		"calendar": "Work",
	}
	res, err := updateEvent(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updated map[string]string
	if err := json.Unmarshal([]byte(res), &updated); err != nil {
		t.Fatalf("failed to unmarshal update event response: %v", err)
	}

	if updated["uid"] != "12345" {
		t.Errorf("expected uid 12345, got %s", updated["uid"])
	}
	if updated["calendar"] != "Work" {
		t.Errorf("expected calendar Work, got %s", updated["calendar"])
	}
	if updated["summary"] != "New Title" {
		t.Errorf("expected summary 'New Title', got %s", updated["summary"])
	}
	if updated["startDate"] != "2026-06-02T10:00:00" {
		t.Errorf("expected startDate '2026-06-02T10:00:00', got %s", updated["startDate"])
	}
	if updated["endDate"] != "2026-06-02T11:00:00" {
		t.Errorf("expected endDate '2026-06-02T11:00:00', got %s", updated["endDate"])
	}
}

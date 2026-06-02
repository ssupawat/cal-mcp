package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

// --- JSON-RPC types ---

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// --- helpers ---

var runAppleScript = func(script string) (string, error) {
	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func textResult(text string) interface{} {
	return map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": text},
		},
	}
}

func errorResult(msg string) interface{} {
	return map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": msg},
		},
		"isError": true,
	}
}

// --- MCP protocol ---

func handleRequest(req Request) *Response {
	if req.ID == nil {
		return nil
	}

	switch req.Method {
	case "initialize":
		return &Response{
			JSONRPC: "2.0", ID: req.ID,
			Result: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
				"serverInfo":      map[string]interface{}{"name": "cal-mcp", "version": "1.0.0"},
			},
		}

	case "ping":
		return &Response{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{}}

	case "tools/list":
		return &Response{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{"tools": toolDefs()}}

	case "tools/call":
		return handleToolCall(req)

	default:
		return &Response{
			JSONRPC: "2.0", ID: req.ID,
			Error: &RPCError{Code: -32601, Message: "method not found: " + req.Method},
		}
	}
}

// --- Tool definitions ---

func toolDefs() []map[string]interface{} {
	prop := func(desc, typ string) map[string]interface{} {
		return map[string]interface{}{"type": typ, "description": desc}
	}

	return []map[string]interface{}{
		{
			"name":        "list-calendars",
			"description": "List all available macOS calendars",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "list-events",
			"description": "List calendar events in a date range",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"start":    prop("Start date in ISO 8601 (e.g. 2025-06-01T00:00:00)", "string"),
					"end":      prop("End date in ISO 8601 (e.g. 2025-06-30T23:59:59)", "string"),
					"calendar": prop("Calendar name (optional, defaults to all)", "string"),
				},
				"required": []string{"start", "end"},
			},
		},
		{
			"name":        "create-event",
			"description": "Create a new calendar event",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title":    prop("Event title", "string"),
					"start":    prop("Start date/time in ISO 8601", "string"),
					"end":      prop("End date/time in ISO 8601", "string"),
					"calendar": prop("Calendar name", "string"),
				},
				"required": []string{"title", "start", "end", "calendar"},
			},
		},
		{
			"name":        "delete-event",
			"description": "Delete an event by UID",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"uid": prop("Event UID to delete", "string"),
				},
				"required": []string{"uid"},
			},
		},
		{
			"name":        "update-event",
			"description": "Update an existing calendar event by UID. Only specified fields will be changed.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"uid":      prop("Event UID to update", "string"),
					"title":    prop("New event title (optional)", "string"),
					"start":    prop("New start date/time in ISO 8601 (optional)", "string"),
					"end":      prop("New end date/time in ISO 8601 (optional)", "string"),
					"calendar": prop("New calendar name to move the event to (optional)", "string"),
				},
				"required": []string{"uid"},
			},
		},
	}
}

// --- Tool dispatch ---

type toolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

func handleToolCall(req Request) *Response {
	var p toolParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return &Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32602, Message: "invalid params"}}
		}
	}

	var result string
	var err error

	switch p.Name {
	case "list-calendars":
		result, err = listCalendars()
	case "list-events":
		result, err = listEvents(p.Arguments)
	case "create-event":
		result, err = createEvent(p.Arguments)
	case "delete-event":
		result, err = deleteEvent(p.Arguments)
	case "update-event":
		result, err = updateEvent(p.Arguments)
	default:
		return &Response{
			JSONRPC: "2.0", ID: req.ID,
			Error: &RPCError{Code: -32602, Message: "unknown tool: " + p.Name},
		}
	}

	if err != nil {
		return &Response{JSONRPC: "2.0", ID: req.ID, Result: errorResult(err.Error())}
	}
	return &Response{JSONRPC: "2.0", ID: req.ID, Result: textResult(result)}
}

// --- Tool implementations (AppleScript) ---

func listCalendars() (string, error) {
	script := `
tell application "Calendar"
	set output to ""
	repeat with c in calendars
		set output to output & name of c & linefeed
	end repeat
	return output
end tell
`
	out, err := runAppleScript(script)
	if err != nil {
		return "", err
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	names := make([]string, 0, len(lines))
	for _, l := range lines {
		if l != "" {
			names = append(names, l)
		}
	}

	b, _ := json.Marshal(names)
	return string(b), nil
}

func listEvents(args map[string]interface{}) (string, error) {
	start, _ := args["start"].(string)
	end, _ := args["end"].(string)
	if start == "" || end == "" {
		return "", fmt.Errorf("start and end are required")
	}

	calName, _ := args["calendar"].(string)

	script := `
tell application "Calendar"
	set lo to date "` + start + `"
	set hi to date "` + end + `"
	set output to ""
`

	if calName != "" {
		script += `
	set cals to {calendar "` + calName + `"}
`
	} else {
		script += `
	set cals to calendars
`
	}

	script += `
	repeat with c in cals
		set calName to name of c
		try
			set evts to (every event of c whose start date >= lo and start date <= hi)
			repeat with e in evts
				set eStart to start date of e as «class isot»
				set eEnd to end date of e as «class isot»
				set output to output & calName & "|" & summary of e & "|" & uid of e & "|" & eStart & "|" & eEnd & linefeed
			end repeat
		end try
	end repeat
	return output
end tell
`

	out, err := runAppleScript(script)
	if err != nil {
		return "", err
	}

	type event struct {
		UID       string `json:"uid"`
		Summary   string `json:"summary"`
		StartDate string `json:"startDate"`
		EndDate   string `json:"endDate"`
		Calendar  string `json:"calendar"`
	}

	var events []event
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, l := range lines {
		if l == "" {
			continue
		}
		parts := strings.SplitN(l, "|", 5)
		if len(parts) < 5 {
			continue
		}
		events = append(events, event{
			Calendar:  parts[0],
			Summary:   parts[1],
			UID:       parts[2],
			StartDate: parts[3],
			EndDate:   parts[4],
		})
	}

	if events == nil {
		events = []event{}
	}

	b, _ := json.Marshal(events)
	return string(b), nil
}

func createEvent(args map[string]interface{}) (string, error) {
	title, _ := args["title"].(string)
	start, _ := args["start"].(string)
	end, _ := args["end"].(string)
	calName, _ := args["calendar"].(string)

	if title == "" || start == "" || end == "" || calName == "" {
		return "", fmt.Errorf("title, start, end, and calendar are required")
	}

	script := `
tell application "Calendar"
	set cal to calendar "` + calName + `"
	set startD to date "` + start + `"
	set endD to date "` + end + `"
	set newEvt to make new event at end of events of cal with properties {summary:"` + title + `", start date:startD, end date:endD}
	set eStart to start date of newEvt as «class isot»
	set eEnd to end date of newEvt as «class isot»
	return uid of newEvt & "|" & summary of newEvt & "|" & eStart & "|" & eEnd
end tell
`

	out, err := runAppleScript(script)
	if err != nil {
		return "", err
	}

	parts := strings.SplitN(out, "|", 4)
	if len(parts) < 4 {
		return "", fmt.Errorf("unexpected response: %s", out)
	}

	b, _ := json.Marshal(map[string]string{
		"uid":       parts[0],
		"summary":   parts[1],
		"startDate": parts[2],
		"endDate":   parts[3],
		"calendar":  calName,
	})
	return string(b), nil
}

func deleteEvent(args map[string]interface{}) (string, error) {
	uid, _ := args["uid"].(string)
	if uid == "" {
		return "", fmt.Errorf("uid is required")
	}

	script := `
tell application "Calendar"
	set targetUid to "` + uid + `"
	set found to false
	repeat with c in calendars
		try
			set evts to (every event of c whose uid is targetUid)
			repeat with e in evts
				delete e
				set found to true
			end repeat
		end try
	end repeat
	return found as text
end tell
`

	out, err := runAppleScript(script)
	if err != nil {
		return "", err
	}

	deleted := strings.TrimSpace(out) == "true"
	b, _ := json.Marshal(map[string]interface{}{
		"deleted": deleted,
		"uid":     uid,
	})
	return string(b), nil
}

func updateEvent(args map[string]interface{}) (string, error) {
	uid, _ := args["uid"].(string)
	if uid == "" {
		return "", fmt.Errorf("uid is required")
	}

	title, hasTitle := args["title"].(string)
	start, hasStart := args["start"].(string)
	end, hasEnd := args["end"].(string)
	calName, hasCal := args["calendar"].(string)

	script := `
tell application "Calendar"
	set targetUid to "` + uid + `"
	set found to false
	set outCal to ""
	set outSummary to ""
	set outStart to ""
	set outEnd to ""
	repeat with c in calendars
		try
			set evts to (every event of c whose uid is targetUid)
			repeat with e in evts
				set outSummary to summary of e
				set outStart to start date of e as «class isot»
				set outEnd to end date of e as «class isot»
				set outCal to name of c
				set found to true
`

	if hasTitle && title != "" {
		script += `
				set summary of e to "` + title + `"
				set outSummary to "` + title + `"
`
	}
	if hasStart && start != "" {
		script += `
				set startD to date "` + start + `"
				set start date of e to startD
				set outStart to startD as «class isot»
`
	}
	if hasEnd && end != "" {
		script += `
				set endD to date "` + end + `"
				set end date of e to endD
				set outEnd to endD as «class isot»
`
	}
	if hasCal && calName != "" {
		script += `
				if name of c is not "` + calName + `" then
					move e to end of events of calendar "` + calName + `"
					set outCal to "` + calName + `"
				end if
`
	}

	script += `
			end repeat
		end try
	end repeat
	if found then
		return "true" & "|" & outCal & "|" & outSummary & "|" & outStart & "|" & outEnd
	else
		return "false"
	end if
end tell
`

	out, err := runAppleScript(script)
	if err != nil {
		return "", err
	}

	if out == "false" {
		return "", fmt.Errorf("event with UID %s not found", uid)
	}

	parts := strings.SplitN(out, "|", 5)
	if len(parts) < 5 {
		return "", fmt.Errorf("unexpected response: %s", out)
	}

	b, _ := json.Marshal(map[string]string{
		"uid":       uid,
		"calendar":  parts[1],
		"summary":   parts[2],
		"startDate": parts[3],
		"endDate":   parts[4],
	})
	return string(b), nil
}

// --- Main loop ---

func main() {
	log.SetPrefix("cal-mcp: ")
	log.SetOutput(os.Stderr)

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			log.Printf("parse error: %v", err)
			continue
		}

		resp := handleRequest(req)
		if resp == nil {
			continue
		}

		out, err := json.Marshal(resp)
		if err != nil {
			log.Printf("marshal error: %v", err)
			continue
		}
		fmt.Println(string(out))
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("stdin read error: %v", err)
	}
}

# cal-mcp

Apple Calendar MCP server written in Go.

Provides AI agents with access to macOS Calendar via Model Context Protocol.

## Tools

- `list-calendars` — list all calendars
- `list-events` — list events in a date range
- `create-event` — create a new event
- `delete-event` — delete an event

## Usage

```bash
# Build
go build -o cal-mcp .

# Run as MCP server (stdio transport)
cal-mcp
```

## Requirements

- macOS
- Calendar access granted in System Preferences

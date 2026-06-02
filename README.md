# cal-mcp

Apple Calendar MCP server written in Go.

Provides AI agents with access to macOS Calendar via Model Context Protocol.

## Tools

### `list-calendars`
List all available macOS calendars.

No arguments required.

### `list-events`
List calendar events within a date range.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `start` | string | yes | Start date in ISO 8601 (`2025-06-01`, `2025-06-01T09:00:00`, or `2025-06-01T09:00:00+07:00`) |
| `end` | string | yes | End date in ISO 8601 |
| `calendar` | string | no | Calendar name to filter by (defaults to all calendars) |

### `create-event`
Create a new calendar event.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `title` | string | yes | Event title |
| `start` | string | yes | Start date/time in ISO 8601 |
| `end` | string | yes | End date/time in ISO 8601 |
| `calendar` | string | yes | Target calendar name |

### `delete-event`
Delete an event by its UID.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `uid` | string | yes | Event UID to delete |

## Date handling

Dates are parsed from ISO 8601 and converted to locale-safe AppleScript dates via a `buildDate` helper. This avoids locale-specific string parsing issues when driving Calendar.app.

## Usage

```bash
# Build
go build -o cal-mcp .

# Run as MCP server (stdio transport)
./cal-mcp
```

## Requirements

- macOS
- Calendar access granted in System Preferences

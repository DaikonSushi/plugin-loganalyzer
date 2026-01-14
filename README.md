# Log Analyzer Plugin

AI-powered log analysis plugin for bot-platform using knot-cli.

## Features

- 🔍 **AI-Powered Analysis**: Uses knot-cli to analyze logs with AI capabilities
- 📋 **Task ID Tracking**: Each analysis request gets a unique ID for easy reference
- ⏱️ **Duration Tracking**: Records analysis time for debugging and performance monitoring
- 📁 **Shared Output**: Results saved to shared_data directory (accessible by napcat)
- 🔄 **Concurrent Support**: Handles multiple analysis requests simultaneously
- 📊 **Status Tracking**: Check the status of pending/running/completed tasks

## Installation

### Using botctl
```bash
./botctl install https://github.com/DaikonSushi/plugin-loganalyzer
```

### Manual Installation
1. Download the appropriate binary for your platform from the releases page
2. Place the binary in your `plugins-bin` directory
3. Create configuration file in `plugins-config/loganalyzer.json`

## Configuration

Create a configuration file at `plugins-config/loganalyzer.json`:

```json
{
  "name": "loganalyzer",
  "enabled": true,
  "binary": "loganalyzer-plugin",
  "description": "AI-powered log analysis plugin using knot-cli",
  "version": "1.0.0",
  "author": "hovanzhang",
  "commands": [
    "analyze",
    "analyzestatus",
    "analyzehelp"
  ],
  "env": {
    "KNOT_CLI_PATH": "knot-cli",
    "WORKSPACE_PATH": "/path/to/your/codebase",
    "SYSTEM_PROMPT_PATH": "/path/to/lookup_rule.md",
    "SHARED_DATA_PATH": "/app/shared_data"
  }
}
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `KNOT_CLI_PATH` | Path to knot-cli binary | `knot-cli` |
| `WORKSPACE_PATH` | Codebase workspace for analysis (required) | - |
| `SYSTEM_PROMPT_PATH` | Path to system prompt file | - |
| `SHARED_DATA_PATH` | Output directory shared with napcat | `/app/shared_data` |

## Usage

### Commands

#### `/analyzehelp`
Show help information about the plugin.

#### `/analyze <log_content>`
Analyze the given log content using AI.

Example:
```
/analyze [component] sendRequest request: data={"caller_info":...} response header: map[...] response is: {"errno":1000018,"error":"lacking of resource..."}
```

Response:
```
🔍 Analysis Task Created
━━━━━━━━━━━━━━━━━━━━
📋 Task ID: A1B2C3D4
📝 Log Length: 512 chars
⏳ Status: Queued for analysis...

Use /analyzestatus A1B2C3D4 to check progress
```

#### `/analyzestatus [task_id]`
Check the status of analysis tasks.

Without task_id - shows all your tasks:
```
📊 Your Analysis Tasks
━━━━━━━━━━━━━━━━━━━━
✅ A1B2C3D4: completed
🔄 E5F6G7H8: running
⏳ I9J0K1L2: pending
```

With task_id - shows detailed status:
```
📊 Task Status
━━━━━━━━━━━━━━━━━━━━
📋 Task ID: A1B2C3D4
✅ Status: completed
⏱️  Duration: 45.2s
```

## Workflow

1. User sends `/analyze <log_content>` in chat
2. Plugin generates unique task ID (e.g., `A1B2C3D4`)
3. Plugin queues the analysis and immediately responds with task ID
4. In background:
   - Plugin executes: `knot-cli chat -w <workspace> --system-prompt <prompt> -p "<log>" --codebase`
   - Output is redirected to `shared_data/analysis_<task_id>.txt`
5. When complete, plugin sends result back to user with:
   - Task ID for reference
   - Analysis duration
   - Analysis result (truncated if too long)
   - Full file uploaded if result was truncated

## Docker Setup

Make sure to mount the shared_data volume in your docker-compose:

```yaml
services:
  bot-platform:
    volumes:
      - shared_data:/app/shared_data
    environment:
      - KNOT_CLI_PATH=/usr/local/bin/knot-cli
      - WORKSPACE_PATH=/data/codebase
      - SYSTEM_PROMPT_PATH=/data/prompts/lookup_rule.md
      - SHARED_DATA_PATH=/app/shared_data

  napcat:
    volumes:
      - shared_data:/app/napcat/shared_data

volumes:
  shared_data:
```

## Building from Source

```bash
# Clone the repository
git clone https://github.com/DaikonSushi/plugin-loganalyzer
cd plugin-loganalyzer

# Build
go mod tidy
go build -ldflags="-s -w" -o loganalyzer-plugin .
```

## License

MIT License

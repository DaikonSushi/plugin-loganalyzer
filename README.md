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
  "version": "1.0.0",
  "description": "AI-powered log analysis plugin using knot-cli",
  "author": "hovanzhang",
  "commands": [
    "analyze",
    "analyzestatus",
    "analyzehelp"
  ],
  "binary_name": "loganalyzer-plugin"
}
```

**Note**: Environment variables (like `KNOT_CLI_PATH`, `WORKSPACE_PATH`, etc.) should be set at the Docker container level or shell environment, not in the plugin JSON config.

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

Since `knot-cli` typically cannot run inside Docker containers (due to dependencies, licensing, or environment requirements), the plugin supports **Proxy Mode** - calling a lightweight HTTP service running on the host machine.

### Recommended: Proxy Mode (for Docker)

This mode runs `knot-cli` on the host machine via an HTTP proxy service.

#### Step 1: Start knot-proxy on Host Machine

```bash
# Set environment variables
export KNOT_CLI_PATH=/usr/local/bin/knot-cli
export WORKSPACE_PATH=/path/to/your/codebase
export SYSTEM_PROMPT_PATH=/path/to/lookup_rule.md
export SHARED_DATA_PATH=/path/to/napcat/shared-data

# Start the proxy
cd knot-proxy
./start-knot-proxy.sh
```

Or run manually:
```bash
go build -o knot-proxy .
./knot-proxy \
  -knot-cli=/usr/local/bin/knot-cli \
  -workspace=/path/to/your/codebase \
  -system-prompt=/path/to/lookup_rule.md \
  -shared-data=/path/to/napcat/shared-data \
  -listen=:9999
```

#### Step 2: Configure docker-compose.yaml

```yaml
services:
  bot-platform:
    image: docker.io/daikonsushi/bot-platform:latest
    volumes:
      - ./bot-platform/plugins-bin:/app/plugins-bin
      - ./bot-platform/plugins-config:/app/plugins-config
      - ./shared-data:/shared-data
    environment:
      # Use proxy mode to call knot-cli on host
      - LOGANALYZER_MODE=proxy
      - KNOT_PROXY_URL=http://host.docker.internal:9999
      - SHARED_DATA_PATH=/shared-data
    # Required for Linux to access host
    extra_hosts:
      - "host.docker.internal:host-gateway"

  napcat:
    volumes:
      - ./shared-data:/shared-data
```

#### Architecture Diagram

```
┌────────────────────────────────────────────────────────────────┐
│                          Host Machine                          │
│                                                                │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │              knot-proxy (HTTP Service)                   │ │
│  │                 Listen: localhost:9999                   │ │
│  │                                                          │ │
│  │   POST /analyze  ──────►  Execute knot-cli chat ...      │ │
│  │   GET /status/:id ─────►  Check analysis status          │ │
│  └──────────────────────────────────────────────────────────┘ │
│                              ▲                                 │
│                              │ HTTP                            │
│  ┌───────────────────────────┴──────────────────────────────┐ │
│  │              Docker: bot-platform                        │ │
│  │   loganalyzer-plugin ──► HTTP host.docker.internal:9999  │ │
│  └──────────────────────────────────────────────────────────┘ │
│                              │                                 │
│                              │ /shared-data volume             │
│                              ▼                                 │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │              Docker: napcat                              │ │
│  │         Read /shared-data/analysis_XXX.txt               │ │
│  └──────────────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────────────────┘
```

### Alternative: Direct Mode (if knot-cli can run in container)

If you can run knot-cli inside the Docker container:

```yaml
services:
  bot-platform:
    volumes:
      - /usr/local/bin/knot-cli:/app/bin/knot-cli:ro
      - /path/to/your/codebase:/app/workspace:ro
    environment:
      - LOGANALYZER_MODE=direct
      - KNOT_CLI_PATH=/app/bin/knot-cli
      - WORKSPACE_PATH=/app/workspace
      - SHARED_DATA_PATH=/shared-data
```

**Note**: The knot-cli binary must be compatible with Alpine Linux (the container OS).

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `LOGANALYZER_MODE` | `proxy` or `direct` | `proxy` |
| `KNOT_PROXY_URL` | URL to knot-proxy service (proxy mode) | `http://host.docker.internal:9999` |
| `KNOT_CLI_PATH` | Path to knot-cli binary (direct mode) | `knot-cli` |
| `WORKSPACE_PATH` | Codebase workspace (direct mode only) | - |
| `SYSTEM_PROMPT_PATH` | System prompt file (direct mode only) | - |
| `SHARED_DATA_PATH` | Output directory shared with napcat | `/shared-data` |

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

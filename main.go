// plugin-loganalyzer - A log analysis plugin using knot-cli for AI-powered log analysis
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/DaikonSushi/bot-platform/pkg/pluginsdk"
	"github.com/google/uuid"
)

// Config holds plugin configuration
type Config struct {
	// KnotCLIPath is the path to knot-cli binary
	KnotCLIPath string `json:"knot_cli_path"`
	// WorkspacePath is the codebase workspace path for knot-cli
	WorkspacePath string `json:"workspace_path"`
	// SystemPromptPath is the path to system prompt file for knot-cli
	SystemPromptPath string `json:"system_prompt_path"`
	// SharedDataPath is the shared directory between napcat and bot-platform
	SharedDataPath string `json:"shared_data_path"`
	// MaxConcurrent is the maximum number of concurrent analysis tasks
	MaxConcurrent int `json:"max_concurrent"`
	// Timeout is the maximum time for each analysis task (in seconds)
	Timeout int `json:"timeout"`
}

// TaskStatus represents the status of an analysis task
type TaskStatus struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"` // "pending", "running", "completed", "failed"
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time,omitempty"`
	Duration  string    `json:"duration,omitempty"`
	Error     string    `json:"error,omitempty"`
	UserID    int64     `json:"user_id"`
	GroupID   int64     `json:"group_id"`
}

// LogAnalyzerPlugin provides AI-powered log analysis using knot-cli
type LogAnalyzerPlugin struct {
	bot       *pluginsdk.BotClient
	config    Config
	tasks     map[string]*TaskStatus
	taskMutex sync.RWMutex
	semaphore chan struct{}
}

// DefaultConfig returns default configuration
func DefaultConfig() Config {
	return Config{
		KnotCLIPath:      "knot-cli",
		WorkspacePath:    "",
		SystemPromptPath: "",
		SharedDataPath:   "/app/shared_data",
		MaxConcurrent:    3,
		Timeout:          300, // 5 minutes
	}
}

// Info returns plugin metadata
func (p *LogAnalyzerPlugin) Info() pluginsdk.PluginInfo {
	return pluginsdk.PluginInfo{
		Name:              "loganalyzer",
		Version:           "1.0.0",
		Description:       "AI-powered log analysis plugin using knot-cli",
		Author:            "hovanzhang",
		Commands:          []string{"analyze", "analyzestatus", "analyzehelp"},
		HandleAllMessages: false,
	}
}

// OnStart is called when the plugin starts
func (p *LogAnalyzerPlugin) OnStart(bot *pluginsdk.BotClient) error {
	p.bot = bot
	p.tasks = make(map[string]*TaskStatus)

	// Load configuration from environment or use defaults
	p.config = DefaultConfig()

	// Override from environment variables if set
	if v := os.Getenv("KNOT_CLI_PATH"); v != "" {
		p.config.KnotCLIPath = v
	}
	if v := os.Getenv("WORKSPACE_PATH"); v != "" {
		p.config.WorkspacePath = v
	}
	if v := os.Getenv("SYSTEM_PROMPT_PATH"); v != "" {
		p.config.SystemPromptPath = v
	}
	if v := os.Getenv("SHARED_DATA_PATH"); v != "" {
		p.config.SharedDataPath = v
	}

	// Initialize semaphore for concurrency control
	p.semaphore = make(chan struct{}, p.config.MaxConcurrent)

	// Ensure shared data directory exists
	if err := os.MkdirAll(p.config.SharedDataPath, 0755); err != nil {
		bot.Log("warn", fmt.Sprintf("Failed to create shared data directory: %v", err))
	}

	bot.Log("info", fmt.Sprintf("Log analyzer plugin started with config: workspace=%s, shared_data=%s",
		p.config.WorkspacePath, p.config.SharedDataPath))

	return nil
}

// OnStop is called when the plugin stops
func (p *LogAnalyzerPlugin) OnStop() error {
	return nil
}

// OnMessage handles incoming messages
func (p *LogAnalyzerPlugin) OnMessage(ctx context.Context, bot *pluginsdk.BotClient, msg *pluginsdk.Message) bool {
	return false
}

// OnCommand handles commands
func (p *LogAnalyzerPlugin) OnCommand(ctx context.Context, bot *pluginsdk.BotClient, cmd string, args []string, msg *pluginsdk.Message) bool {
	switch cmd {
	case "analyzehelp":
		p.handleHelp(bot, msg)
		return true
	case "analyze":
		p.handleAnalyze(ctx, bot, args, msg)
		return true
	case "analyzestatus":
		p.handleStatus(bot, args, msg)
		return true
	}
	return false
}

// handleHelp shows plugin help information
func (p *LogAnalyzerPlugin) handleHelp(bot *pluginsdk.BotClient, msg *pluginsdk.Message) {
	bot.Reply(msg,
		pluginsdk.Text("🔍 Log Analyzer Plugin\n"),
		pluginsdk.Text("━━━━━━━━━━━━━━━━━━━━\n"),
		pluginsdk.Text("AI-powered log analysis using knot-cli\n\n"),
		pluginsdk.Text("Available Commands:\n\n"),
		pluginsdk.Text("📊 /analyze <log_content>\n"),
		pluginsdk.Text("   Analyze the given log content using AI\n"),
		pluginsdk.Text("   The log content should be the error log\n"),
		pluginsdk.Text("   you want to analyze\n\n"),
		pluginsdk.Text("📋 /analyzestatus [task_id]\n"),
		pluginsdk.Text("   Check the status of an analysis task\n"),
		pluginsdk.Text("   Without task_id, shows all your tasks\n\n"),
		pluginsdk.Text("❓ /analyzehelp\n"),
		pluginsdk.Text("   Show this help message\n\n"),
		pluginsdk.Text("Example:\n"),
		pluginsdk.Text("  /analyze [component] sendRequest request: ...\n"),
	)
}

// handleAnalyze handles the analyze command
func (p *LogAnalyzerPlugin) handleAnalyze(ctx context.Context, bot *pluginsdk.BotClient, args []string, msg *pluginsdk.Message) {
	if len(args) == 0 {
		bot.Reply(msg,
			pluginsdk.Text("❌ Please provide log content to analyze\n\n"),
			pluginsdk.Text("Usage: /analyze <log_content>\n"),
			pluginsdk.Text("Example: /analyze [component] sendRequest request: ..."),
		)
		return
	}

	// Check configuration
	if p.config.WorkspacePath == "" {
		bot.Reply(msg, pluginsdk.Text("❌ Plugin not properly configured: workspace path not set\nPlease set WORKSPACE_PATH environment variable"))
		return
	}

	// Generate unique task ID
	taskID := generateShortID()
	logContent := strings.Join(args, " ")

	// Create task status
	task := &TaskStatus{
		ID:        taskID,
		Status:    "pending",
		StartTime: time.Now(),
		UserID:    msg.UserID,
		GroupID:   msg.GroupID,
	}

	p.taskMutex.Lock()
	p.tasks[taskID] = task
	p.taskMutex.Unlock()

	// Acknowledge the request
	bot.Reply(msg,
		pluginsdk.Text(fmt.Sprintf("🔍 Analysis Task Created\n")),
		pluginsdk.Text("━━━━━━━━━━━━━━━━━━━━\n"),
		pluginsdk.Text(fmt.Sprintf("📋 Task ID: %s\n", taskID)),
		pluginsdk.Text(fmt.Sprintf("📝 Log Length: %d chars\n", len(logContent))),
		pluginsdk.Text("⏳ Status: Queued for analysis...\n\n"),
		pluginsdk.Text("Use /analyzestatus "+taskID+" to check progress"),
	)

	// Run analysis in background
	go p.runAnalysis(task, logContent, msg)
}

// runAnalysis executes the knot-cli analysis
func (p *LogAnalyzerPlugin) runAnalysis(task *TaskStatus, logContent string, msg *pluginsdk.Message) {
	// Acquire semaphore for concurrency control
	p.semaphore <- struct{}{}
	defer func() { <-p.semaphore }()

	// Update status to running
	p.taskMutex.Lock()
	task.Status = "running"
	p.taskMutex.Unlock()

	// Create output file path
	outputFileName := fmt.Sprintf("analysis_%s.txt", task.ID)
	outputPath := filepath.Join(p.config.SharedDataPath, outputFileName)

	// Build knot-cli command
	cmdArgs := []string{"chat"}

	if p.config.WorkspacePath != "" {
		cmdArgs = append(cmdArgs, "-w", p.config.WorkspacePath)
	}

	if p.config.SystemPromptPath != "" {
		cmdArgs = append(cmdArgs, "--system-prompt", p.config.SystemPromptPath)
	}

	cmdArgs = append(cmdArgs, "-p", logContent, "--codebase")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(p.config.Timeout)*time.Second)
	defer cancel()

	// Execute knot-cli command
	cmd := exec.CommandContext(ctx, p.config.KnotCLIPath, cmdArgs...)

	// Create output file
	outputFile, err := os.Create(outputPath)
	if err != nil {
		p.completeTask(task, "", fmt.Errorf("failed to create output file: %v", err), msg)
		return
	}

	// Set up pipes
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		outputFile.Close()
		p.completeTask(task, outputPath, fmt.Errorf("failed to create stdout pipe: %v", err), msg)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		outputFile.Close()
		p.completeTask(task, outputPath, fmt.Errorf("failed to create stderr pipe: %v", err), msg)
		return
	}

	// Start command
	if err := cmd.Start(); err != nil {
		outputFile.Close()
		p.completeTask(task, outputPath, fmt.Errorf("failed to start knot-cli: %v", err), msg)
		return
	}

	// Collect output
	var outputBuilder strings.Builder

	// Read stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			outputBuilder.WriteString(line + "\n")
			outputFile.WriteString(line + "\n")
		}
	}()

	// Read stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			// Filter out progress messages, keep only important ones
			if !strings.HasPrefix(line, "[") || strings.Contains(line, "错误") || strings.Contains(line, "Error") {
				outputBuilder.WriteString(line + "\n")
				outputFile.WriteString(line + "\n")
			}
		}
	}()

	// Wait for command to complete
	err = cmd.Wait()
	outputFile.Close()

	if ctx.Err() == context.DeadlineExceeded {
		p.completeTask(task, outputPath, fmt.Errorf("analysis timed out after %d seconds", p.config.Timeout), msg)
		return
	}

	if err != nil {
		p.completeTask(task, outputPath, fmt.Errorf("knot-cli error: %v", err), msg)
		return
	}

	p.completeTask(task, outputPath, nil, msg)
}

// completeTask finalizes the task and sends result to user
func (p *LogAnalyzerPlugin) completeTask(task *TaskStatus, outputPath string, err error, msg *pluginsdk.Message) {
	task.EndTime = time.Now()
	task.Duration = task.EndTime.Sub(task.StartTime).Round(time.Millisecond).String()

	if err != nil {
		task.Status = "failed"
		task.Error = err.Error()

		p.taskMutex.Lock()
		p.tasks[task.ID] = task
		p.taskMutex.Unlock()

		p.bot.Reply(msg,
			pluginsdk.Text(fmt.Sprintf("❌ Analysis Failed\n")),
			pluginsdk.Text("━━━━━━━━━━━━━━━━━━━━\n"),
			pluginsdk.Text(fmt.Sprintf("📋 Task ID: %s\n", task.ID)),
			pluginsdk.Text(fmt.Sprintf("⏱️  Duration: %s\n", task.Duration)),
			pluginsdk.Text(fmt.Sprintf("❌ Error: %s", task.Error)),
		)
		return
	}

	task.Status = "completed"

	p.taskMutex.Lock()
	p.tasks[task.ID] = task
	p.taskMutex.Unlock()

	// Read analysis result
	result, readErr := os.ReadFile(outputPath)
	if readErr != nil {
		p.bot.Reply(msg,
			pluginsdk.Text(fmt.Sprintf("⚠️ Analysis completed but failed to read result\n")),
			pluginsdk.Text(fmt.Sprintf("📋 Task ID: %s\n", task.ID)),
			pluginsdk.Text(fmt.Sprintf("📁 Output File: %s\n", outputPath)),
			pluginsdk.Text(fmt.Sprintf("❌ Read Error: %s", readErr.Error())),
		)
		return
	}

	resultStr := string(result)

	// Extract requestID if present
	requestID := extractRequestID(resultStr)

	// Truncate result if too long for chat message
	const maxLength = 3000
	truncated := false
	if len(resultStr) > maxLength {
		resultStr = resultStr[:maxLength] + "\n\n... [Result truncated, see full output in file]"
		truncated = true
	}

	// Send result
	replyParts := []pluginsdk.MessageSegment{
		pluginsdk.Text("✅ Analysis Completed\n"),
		pluginsdk.Text("━━━━━━━━━━━━━━━━━━━━\n"),
		pluginsdk.Text(fmt.Sprintf("📋 Task ID: %s\n", task.ID)),
		pluginsdk.Text(fmt.Sprintf("⏱️  Duration: %s\n", task.Duration)),
	}

	if requestID != "" {
		replyParts = append(replyParts, pluginsdk.Text(fmt.Sprintf("🔑 Request ID: %s\n", requestID)))
	}

	replyParts = append(replyParts,
		pluginsdk.Text(fmt.Sprintf("📁 Output File: %s\n", outputPath)),
		pluginsdk.Text("━━━━━━━━━━━━━━━━━━━━\n\n"),
		pluginsdk.Text(resultStr),
	)

	p.bot.Reply(msg, replyParts...)

	// If truncated, also upload the full file
	if truncated {
		if msg.GroupID > 0 {
			p.bot.UploadGroupFile(msg.GroupID, outputPath, fmt.Sprintf("analysis_%s.txt", task.ID), "/")
		} else {
			p.bot.UploadPrivateFile(msg.UserID, outputPath, fmt.Sprintf("analysis_%s.txt", task.ID))
		}
	}
}

// handleStatus handles the analyzestatus command
func (p *LogAnalyzerPlugin) handleStatus(bot *pluginsdk.BotClient, args []string, msg *pluginsdk.Message) {
	p.taskMutex.RLock()
	defer p.taskMutex.RUnlock()

	if len(args) > 0 {
		// Show specific task status
		taskID := args[0]
		task, exists := p.tasks[taskID]
		if !exists {
			bot.Reply(msg, pluginsdk.Text(fmt.Sprintf("❌ Task not found: %s", taskID)))
			return
		}

		statusIcon := getStatusIcon(task.Status)
		duration := ""
		if task.Status == "completed" || task.Status == "failed" {
			duration = fmt.Sprintf("\n⏱️  Duration: %s", task.Duration)
		} else {
			duration = fmt.Sprintf("\n⏱️  Running: %s", time.Since(task.StartTime).Round(time.Second).String())
		}

		errorMsg := ""
		if task.Error != "" {
			errorMsg = fmt.Sprintf("\n❌ Error: %s", task.Error)
		}

		bot.Reply(msg,
			pluginsdk.Text(fmt.Sprintf("📊 Task Status\n")),
			pluginsdk.Text("━━━━━━━━━━━━━━━━━━━━\n"),
			pluginsdk.Text(fmt.Sprintf("📋 Task ID: %s\n", task.ID)),
			pluginsdk.Text(fmt.Sprintf("%s Status: %s%s%s", statusIcon, task.Status, duration, errorMsg)),
		)
		return
	}

	// Show all user's tasks
	var userTasks []*TaskStatus
	for _, task := range p.tasks {
		if task.UserID == msg.UserID {
			userTasks = append(userTasks, task)
		}
	}

	if len(userTasks) == 0 {
		bot.Reply(msg, pluginsdk.Text("📊 You have no analysis tasks"))
		return
	}

	response := "📊 Your Analysis Tasks\n━━━━━━━━━━━━━━━━━━━━\n"
	for _, task := range userTasks {
		statusIcon := getStatusIcon(task.Status)
		response += fmt.Sprintf("%s %s: %s\n", statusIcon, task.ID, task.Status)
	}

	bot.Reply(msg, pluginsdk.Text(response))
}

// generateShortID generates a short unique ID
func generateShortID() string {
	id := uuid.New().String()
	// Return first 8 characters for brevity
	return strings.ToUpper(id[:8])
}

// getStatusIcon returns emoji for status
func getStatusIcon(status string) string {
	switch status {
	case "pending":
		return "⏳"
	case "running":
		return "🔄"
	case "completed":
		return "✅"
	case "failed":
		return "❌"
	default:
		return "❓"
	}
}

// extractRequestID extracts requestID from analysis result
func extractRequestID(result string) string {
	// Look for requestID pattern in the result
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), "requestid") {
			// Try to extract the ID value
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				return strings.TrimSpace(parts[len(parts)-1])
			}
		}
	}
	return ""
}

func main() {
	pluginsdk.Run(&LogAnalyzerPlugin{})
}

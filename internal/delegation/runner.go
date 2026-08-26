package delegation

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const maxRequestBytes = 256 * 1024
const maxOutputBytes = 1024 * 1024

type AuthorityProof struct {
	ClaimConfirmed bool `json:"claim_confirmed"`
	LeaseConfirmed bool `json:"lease_confirmed"`
}

// Request is a transient handoff document. The bridge deletes it before AGY
// starts; only its objective digest and operational metadata enter SQLite.
type Request struct {
	Role         string          `json:"role"`
	TaskID       string          `json:"task_id,omitempty"`
	Objective    string          `json:"objective"`
	Workspace    string          `json:"workspace"`
	Worktree     string          `json:"worktree,omitempty"`
	AllowedFiles []string        `json:"allowed_files,omitempty"`
	Authority    AuthorityProof  `json:"authority"`
	OutputSchema json.RawMessage `json:"output_schema,omitempty"`
}

func ReadRequest(path string) (Request, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return Request{}, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > maxRequestBytes {
		return Request{}, errors.New("delegation request must be a regular file no larger than 256 KiB")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Request{}, err
	}
	var request Request
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return Request{}, fmt.Errorf("decode delegation request: %w", err)
	}
	if err := request.Validate(); err != nil {
		return Request{}, err
	}
	return request, nil
}

func (r Request) Validate() error {
	if !supportedRoles[r.Role] {
		return fmt.Errorf("unsupported role %q", r.Role)
	}
	if strings.TrimSpace(r.Objective) == "" || len(r.Objective) > 128*1024 {
		return errors.New("objective is required and must not exceed 128 KiB")
	}
	workspace, err := filepath.Abs(r.Workspace)
	if err != nil || workspace != filepath.Clean(r.Workspace) {
		return errors.New("workspace must be an absolute normalized path")
	}
	info, err := os.Stat(workspace)
	if err != nil || !info.IsDir() {
		return errors.New("workspace must be an existing directory")
	}
	if r.Role == "implement" {
		if !r.Authority.ClaimConfirmed || !r.Authority.LeaseConfirmed {
			return errors.New("implement delegation requires a confirmed cortex-ia work claim and file lease")
		}
		if r.Worktree == "" {
			return errors.New("implement delegation requires worktree")
		}
		worktree, err := filepath.Abs(r.Worktree)
		if err != nil || worktree != filepath.Clean(r.Worktree) {
			return errors.New("worktree must be an absolute normalized path")
		}
		info, err := os.Stat(worktree)
		if err != nil || !info.IsDir() {
			return errors.New("worktree must be an existing directory")
		}
	}
	for _, allowed := range r.AllowedFiles {
		if filepath.IsAbs(allowed) || allowed == "." || strings.HasPrefix(filepath.Clean(allowed), ".."+string(filepath.Separator)) {
			return fmt.Errorf("unsafe allowed file %q", allowed)
		}
	}
	if len(r.OutputSchema) > 64*1024 || (len(r.OutputSchema) > 0 && !json.Valid(r.OutputSchema)) {
		return errors.New("output_schema must be valid JSON no larger than 64 KiB")
	}
	return nil
}

func ObjectiveDigest(objective string) string {
	digest := sha256.Sum256([]byte(objective))
	return "sha256:" + hex.EncodeToString(digest[:])
}

func CreateFromRequest(ctx context.Context, store *Store, request Request, transport string) (Job, error) {
	if err := request.Validate(); err != nil {
		return Job{}, err
	}
	return store.Create(ctx, NewJob{
		Role: request.Role, TaskID: request.TaskID, ObjectiveDigest: ObjectiveDigest(request.Objective),
		Transport: transport, Workspace: request.Workspace, Worktree: request.Worktree,
	})
}

func RunWorker(ctx context.Context, home, id, requestPath string) error {
	request, err := ReadRequest(requestPath)
	if err != nil {
		return err
	}
	if err := os.Remove(requestPath); err != nil {
		return fmt.Errorf("delete transient delegation request: %w", err)
	}
	_ = os.Remove(filepath.Dir(requestPath))
	store, err := OpenStore(DefaultDBPath(home))
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()
	job, err := store.Get(ctx, id)
	if err != nil {
		return err
	}
	if job.Role != request.Role || job.ObjectiveDigest != ObjectiveDigest(request.Objective) {
		return errors.New("delegation request does not match accepted job")
	}
	cfg, err := Load(filepath.Join(home, ".config", "opencode"))
	if err != nil {
		return fmt.Errorf("load delegation config: %w", err)
	}
	role := cfg.Roles[request.Role]
	if !cfg.DelegationEnabled || !role.Delegate || role.CLI != "agy" {
		return errors.New("external delegation is not enabled for this role")
	}
	timeout := time.Duration(cfg.HerdrSettings.TimeoutSeconds) * time.Second
	owner, err := newID()
	if err != nil {
		return err
	}
	if err := store.Claim(ctx, id, owner, os.Getpid(), timeout+30*time.Second); err != nil {
		return err
	}
	if err := store.MarkRunning(ctx, id); err != nil {
		return err
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	watchDone := make(chan struct{})
	defer close(watchDone)
	go watchCancellation(runCtx, store, id, cancel, watchDone)
	output, exitCode, runErr := runAGY(runCtx, request, role, timeout)
	if current, getErr := store.Get(context.Background(), id); getErr == nil && current.Status == StatusCancelled {
		return nil
	}
	hash := sha256.Sum256(output)
	receipt := Receipt{Output: normalizeJSON(output), OutputHash: "sha256:" + hex.EncodeToString(hash[:]), ExitCode: exitCode}
	if runErr == nil {
		return store.Complete(ctx, id, StatusSucceeded, receipt, "", "")
	}
	status, code := StatusFailed, "AGY_FAILED"
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		status, code = StatusTimedOut, "TIMEOUT"
	}
	message := runErr.Error()
	if completeErr := store.Complete(context.Background(), id, status, receipt, code, message); completeErr != nil {
		return errors.Join(runErr, completeErr)
	}
	return runErr
}

func watchCancellation(ctx context.Context, store *Store, id string, cancel context.CancelFunc, done <-chan struct{}) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			job, err := store.Get(context.Background(), id)
			if err == nil && (job.Status == StatusCancelled || job.Status == StatusLost) {
				cancel()
				return
			}
		}
	}
}

func runAGY(ctx context.Context, request Request, role RoleConfig, timeout time.Duration) ([]byte, int, error) {
	agy, err := resolveAGY()
	if err != nil {
		return nil, -1, err
	}
	args := []string{
		"--output-format", "stream-json",
		"--print-timeout", timeout.String(),
		"--dangerously-skip-permissions",
		"--disable-slash-commands",
	}
	if role.Mode != "" {
		args = append(args, "--mode", role.Mode)
	}
	schemaPath := ""
	if len(request.OutputSchema) > 0 {
		file, err := os.CreateTemp("", "cortex-ia-schema-*.json")
		if err != nil {
			return nil, -1, err
		}
		schemaPath = file.Name()
		if err := file.Chmod(0o600); err != nil {
			_ = file.Close()
			_ = os.Remove(schemaPath)
			return nil, -1, err
		}
		if _, err := file.Write(request.OutputSchema); err != nil {
			_ = file.Close()
			_ = os.Remove(schemaPath)
			return nil, -1, err
		}
		if err := file.Close(); err != nil {
			_ = os.Remove(schemaPath)
			return nil, -1, err
		}
		defer func() { _ = os.Remove(schemaPath) }()
		args = append(args, "--json-schema", schemaPath)
	}
	args = append(args, "--print", externalPrompt(request))

	cmd := exec.CommandContext(ctx, agy, args...)
	cmd.Dir = request.Workspace
	if request.Role == "implement" {
		cmd.Dir = request.Worktree
	}
	sandboxHome, err := os.MkdirTemp("", "cortex-ia-agy-home-*")
	if err != nil {
		return nil, -1, err
	}
	defer func() { _ = os.RemoveAll(sandboxHome) }()
	cmd.Env = isolatedAGYEnvironment(sandboxHome)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, -1, err
	}
	var stderr limitedBuffer
	cmd.Stderr = &stderr

	fmt.Printf("======================================================================\n")
	fmt.Printf("🚀 [CORTEX-IA] DELEGATED %s WORKER\n", strings.ToUpper(request.Role))
	fmt.Printf("----------------------------------------------------------------------\n")
	fmt.Printf("🆔 Role:       %s\n", request.Role)
	fmt.Printf("⚙️  CLI:        %s\n", role.CLI)
	fmt.Printf("📂 Directory:  %s\n", cmd.Dir)
	fmt.Printf("📋 Objective:  %s\n", bounded(request.Objective, 200))
	fmt.Printf("----------------------------------------------------------------------\n")
	fmt.Println("⚡ Initializing worker session...")
	_ = os.Stdout.Sync()

	startTime := time.Now()
	if err := cmd.Start(); err != nil {
		return nil, -1, err
	}

	var lastActivity time.Time
	var activityMu sync.Mutex
	lastActivity = time.Now()

	doneHeartbeat := make(chan struct{})
	go func() {
		ticker := time.NewTicker(400 * time.Millisecond)
		defer ticker.Stop()
		spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		for {
			select {
			case <-doneHeartbeat:
				return
			case <-ticker.C:
				activityMu.Lock()
				idle := time.Since(lastActivity)
				activityMu.Unlock()
				if idle >= 1200*time.Millisecond {
					elapsed := time.Since(startTime).Round(time.Second)
					fmt.Printf("\r%s [%s] Worker processing task via %s... (%s elapsed)   ", spinner[i%len(spinner)], request.Role, role.CLI, elapsed)
					_ = os.Stdout.Sync()
					i++
				}
			}
		}
	}()

	var finalResultBytes []byte
	var fullResponseText strings.Builder
	scanner := bufio.NewScanner(stdoutPipe)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		activityMu.Lock()
		lastActivity = time.Now()
		activityMu.Unlock()

		var msg struct {
			Event      string          `json:"event"`
			Result     json.RawMessage `json:"result"`
			StepUpdate struct {
				StepType string `json:"step_type"`
				State    string `json:"state"`
				Text     string `json:"text_delta"`
				ToolName string `json:"tool_name"`
				ToolInfo struct {
					Name       string          `json:"name"`
					Parameters json.RawMessage `json:"parameters"`
					Output     string          `json:"output"`
				} `json:"tool_info"`
				Duration float64 `json:"duration_seconds"`
			} `json:"step_update"`
		}
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}
		switch msg.Event {
		case "step_update":
			switch msg.StepUpdate.StepType {
			case "tool", "tool_call":
				name := msg.StepUpdate.ToolName
				if name == "" {
					name = msg.StepUpdate.ToolInfo.Name
				}
				if msg.StepUpdate.State == "ACTIVE" {
					var rawParams map[string]any
					actionSummary := ""
					if len(msg.StepUpdate.ToolInfo.Parameters) > 0 {
						_ = json.Unmarshal(msg.StepUpdate.ToolInfo.Parameters, &rawParams)
					}
					if s, ok := rawParams["toolAction"].(string); ok && s != "" {
						actionSummary = s
					} else if s, ok := rawParams["toolSummary"].(string); ok && s != "" {
						actionSummary = s
					} else if s, ok := rawParams["Description"].(string); ok && s != "" {
						actionSummary = s
					} else if s, ok := rawParams["CommandLine"].(string); ok && s != "" {
						actionSummary = "Exec: " + s
					} else if s, ok := rawParams["AbsolutePath"].(string); ok && s != "" {
						actionSummary = "Read: " + filepath.Base(s)
					} else if s, ok := rawParams["TargetFile"].(string); ok && s != "" {
						actionSummary = "Edit: " + filepath.Base(s)
					} else if s, ok := rawParams["Query"].(string); ok && s != "" {
						actionSummary = "Grep: " + s
					} else if s, ok := rawParams["Pattern"].(string); ok && s != "" {
						actionSummary = "Find: " + s
					}

					fmt.Printf("\r                                                                               \r")
					if actionSummary != "" {
						fmt.Printf("⚡ [%s] %s (%s)\n", request.Role, actionSummary, name)
					} else {
						paramsSummary := bounded(string(msg.StepUpdate.ToolInfo.Parameters), 160)
						fmt.Printf("🔧 [%s] Tool: %s %s\n", request.Role, name, paramsSummary)
					}
					_ = os.Stdout.Sync()
				} else if msg.StepUpdate.State == "DONE" {
					outSummary := ""
					if msg.StepUpdate.ToolInfo.Output != "" {
						cleanOut := strings.ReplaceAll(msg.StepUpdate.ToolInfo.Output, "\n", " ")
						cleanOut = strings.TrimSpace(cleanOut)
						if cleanOut != "" {
							outSummary = " ➔ " + bounded(cleanOut, 100)
						}
					}
					fmt.Printf("\r                                                                               \r")
					fmt.Printf("   ↳ Done (%.2fs)%s\n", msg.StepUpdate.Duration, outSummary)
					_ = os.Stdout.Sync()
				}
			case "agent_response", "thought":
				if msg.StepUpdate.Text != "" {
					fmt.Printf("\r                                                                               \r")
					fmt.Print(msg.StepUpdate.Text)
					fullResponseText.WriteString(msg.StepUpdate.Text)
					_ = os.Stdout.Sync()
				}
			case "checkpoint":
				fmt.Printf("\r                                                                               \r")
				fmt.Printf("⏱️  [Checkpoint: %.1fs]\n", msg.StepUpdate.Duration)
				_ = os.Stdout.Sync()
			}
		case "result":
			finalResultBytes = msg.Result
		}
	}

	close(doneHeartbeat)
	fmt.Printf("\r                                                                               \r")
	_ = os.Stdout.Sync()

	err = cmd.Wait()
	elapsed := time.Since(startTime).Round(time.Millisecond)

	exitCode := 0
	if err != nil {
		exitCode = -1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
		if stderr.Len() > 0 {
			err = fmt.Errorf("agy exited %d: %s", exitCode, bounded(stderr.String(), 512))
		}
	}

	if len(finalResultBytes) == 0 {
		fallback := map[string]any{
			"status":   "SUCCESS",
			"response": fullResponseText.String(),
		}
		finalResultBytes, _ = json.Marshal(fallback)
	}

	if err == nil {
		fmt.Printf("\n----------------------------------------------------------------------\n")
		fmt.Printf("✅ [CORTEX-IA] Delegated %s task completed in %s (exit code %d)\n", request.Role, elapsed, exitCode)
		printResultSummary(finalResultBytes)
		fmt.Printf("======================================================================\n\n")
	} else {
		fmt.Printf("\n----------------------------------------------------------------------\n")
		fmt.Printf("⚠️ [CORTEX-IA] Delegated %s task failed after %s: %v\n", request.Role, elapsed, err)
		if stderr.Len() > 0 {
			fmt.Printf("🛑 Stderr: %s\n", stderr.String())
		}
		fmt.Printf("======================================================================\n\n")
	}
	return finalResultBytes, exitCode, err
}

func printResultSummary(output []byte) {
	var parsed struct {
		StructuredOutput struct {
			Summary             string `json:"summary"`
			PhaseStatus         string `json:"phase_status"`
			TaskStatus          string `json:"task_status"`
			VerificationVerdict string `json:"verification_verdict"`
		} `json:"structured_output"`
		Usage struct {
			TotalTokens    int `json:"total_tokens"`
			InputTokens    int `json:"input_tokens"`
			OutputTokens   int `json:"output_tokens"`
			ThinkingTokens int `json:"thinking_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(output, &parsed); err == nil {
		if parsed.StructuredOutput.PhaseStatus != "" {
			fmt.Printf("🏷️  Phase Status:  %s\n", parsed.StructuredOutput.PhaseStatus)
		}
		if parsed.StructuredOutput.VerificationVerdict != "" {
			fmt.Printf("⚖️  Verdict:       %s\n", parsed.StructuredOutput.VerificationVerdict)
		}
		if parsed.Usage.TotalTokens > 0 {
			fmt.Printf("📊 Token Usage:   %d (in: %d, out: %d, think: %d)\n",
				parsed.Usage.TotalTokens, parsed.Usage.InputTokens, parsed.Usage.OutputTokens, parsed.Usage.ThinkingTokens)
		}
		if parsed.StructuredOutput.Summary != "" {
			fmt.Printf("📝 Summary:       %s\n", bounded(parsed.StructuredOutput.Summary, 280))
		}
	}
}

func isolatedAGYEnvironment(home string) []string {
	blocked := map[string]bool{"HOME": true, "USERPROFILE": true, "HOMEDRIVE": true, "HOMEPATH": true}
	environment := make([]string, 0, len(os.Environ())+4)
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if !blocked[strings.ToUpper(key)] {
			environment = append(environment, item)
		}
	}
	environment = append(environment, "HOME="+home, "USERPROFILE="+home)
	if volume := filepath.VolumeName(home); volume != "" {
		environment = append(environment, "HOMEDRIVE="+volume, "HOMEPATH="+strings.TrimPrefix(home, volume))
	}
	return environment
}

func externalPrompt(request Request) string {
	var b strings.Builder
	b.WriteString("You are an external leaf executor supervised by cortex-ia. Do not use the cortex-ia work control plane or Cortex MCP, do not start or end a session, and do not delegate or spawn subagents. Return only the requested structured result.\n\n")
	b.WriteString("Role: ")
	b.WriteString(request.Role)
	if request.TaskID != "" {
		b.WriteString("\nTask ID: ")
		b.WriteString(request.TaskID)
	}
	if len(request.AllowedFiles) > 0 {
		b.WriteString("\nAllowed files: ")
		b.WriteString(strings.Join(request.AllowedFiles, ", "))
		b.WriteString("\nDo not modify files outside this list.")
	}
	b.WriteString("\n\nObjective:\n")
	b.WriteString(request.Objective)
	return b.String()
}

func normalizeJSON(output []byte) json.RawMessage {
	output = bytes.TrimSpace(output)
	if len(output) == 0 {
		return json.RawMessage(`{}`)
	}
	if len(output) > maxOutputBytes {
		output = output[:maxOutputBytes]
	}
	if json.Valid(output) {
		return json.RawMessage(output)
	}
	wrapped, _ := json.Marshal(map[string]string{"text": string(output)})
	return wrapped
}

func resolveAGY() (string, error) {
	if path, err := exec.LookPath("agy"); err == nil {
		return path, nil
	}
	if runtime.GOOS == "windows" {
		home, _ := os.UserHomeDir()
		candidates := []string{
			filepath.Join(home, "AppData", "Local", "agy", "bin", "agy.exe"),
			filepath.Join(os.Getenv("LOCALAPPDATA"), "agy", "bin", "agy.exe"),
		}
		for _, candidate := range candidates {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, nil
			}
		}
	}
	return "", errors.New("agy executable not found")
}

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	original := len(p)
	remaining := maxOutputBytes - b.Len()
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = b.Buffer.Write(p)
	}
	return original, nil
}

var _ io.Writer = (*limitedBuffer)(nil)

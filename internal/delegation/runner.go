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
	"sort"
	"strings"
	"sync"
	"time"
)

const maxRequestBytes = 256 * 1024
const maxOutputBytes = 1024 * 1024

const (
	WorkspaceIsolated = "isolated_worktree"
	WorkspaceCurrent  = "current_workspace"
)

var errInvalidReceipt = errors.New("invalid delegation receipt")

// Request is a transient handoff document. The bridge deletes it before AGY
// starts; only its objective digest and operational metadata enter SQLite.
type Request struct {
	Role          string          `json:"role"`
	TaskID        string          `json:"task_id,omitempty"`
	Objective     string          `json:"objective"`
	Workspace     string          `json:"workspace"`
	Worktree      string          `json:"worktree,omitempty"`
	WorkspaceMode string          `json:"workspace_strategy,omitempty"`
	AllowedFiles  []string        `json:"allowed_files,omitempty"`
	OutputSchema  json.RawMessage `json:"output_schema,omitempty"`
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
		if strings.TrimSpace(r.TaskID) == "" {
			return errors.New("implement delegation requires a task id")
		}
		if len(r.AllowedFiles) == 0 {
			return errors.New("implement delegation requires at least one allowed file")
		}
		switch r.WorkspaceMode {
		case WorkspaceIsolated:
			if r.Worktree == "" {
				return errors.New("isolated_worktree strategy requires worktree")
			}
			worktree, err := filepath.Abs(r.Worktree)
			if err != nil || worktree != filepath.Clean(r.Worktree) {
				return errors.New("worktree must be an absolute normalized path")
			}
			info, err := os.Stat(worktree)
			if err != nil || !info.IsDir() {
				return errors.New("worktree must be an existing directory")
			}
			if samePath(workspace, worktree) {
				return errors.New("isolated_worktree must differ from the controller workspace")
			}
			if _, err := os.Lstat(filepath.Join(worktree, ".git")); err != nil {
				return errors.New("isolated_worktree must be a git worktree")
			}
		case WorkspaceCurrent:
			if r.Worktree != "" {
				return errors.New("current_workspace strategy must not include worktree")
			}
		case "":
			return errors.New("implement delegation requires explicit workspace_strategy")
		default:
			return fmt.Errorf("unsupported workspace_strategy %q", r.WorkspaceMode)
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

func (r Request) executionDirectory() string {
	if r.Role == "implement" && r.WorkspaceMode == WorkspaceIsolated {
		return r.Worktree
	}
	return r.Workspace
}

func ObjectiveDigest(objective string) string {
	digest := sha256.Sum256([]byte(objective))
	return "sha256:" + hex.EncodeToString(digest[:])
}

func CreateFromRequest(ctx context.Context, store *Store, request Request, transport string) (Job, error) {
	if err := request.Validate(); err != nil {
		return Job{}, err
	}
	if request.Role == "implement" {
		if request.WorkspaceMode == WorkspaceIsolated {
			if err := validateRelatedWorktree(request.Workspace, request.Worktree); err != nil {
				return Job{}, err
			}
			if err := ensureCleanWorktree(request.Worktree); err != nil {
				return Job{}, err
			}
		}
		if err := store.ValidateDelegationAuthority(ctx, request.TaskID, request.AllowedFiles); err != nil {
			return Job{}, fmt.Errorf("validate implementation authority: %w", err)
		}
	}
	return store.Create(ctx, NewJob{
		Role: request.Role, TaskID: request.TaskID, ObjectiveDigest: ObjectiveDigest(request.Objective),
		Transport: transport, Workspace: request.Workspace, Worktree: request.executionDirectory(),
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
	if request.Role == "implement" {
		if request.WorkspaceMode == WorkspaceIsolated {
			if err := validateRelatedWorktree(request.Workspace, request.Worktree); err != nil {
				return err
			}
			if err := ensureCleanWorktree(request.Worktree); err != nil {
				return err
			}
		}
		if err := store.ValidateDelegationAuthority(ctx, request.TaskID, request.AllowedFiles); err != nil {
			return fmt.Errorf("revalidate implementation authority: %w", err)
		}
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
	if request.TaskID != "" {
		go keepAliveAuthority(runCtx, store, request.TaskID, watchDone)
	}
	output, exitCode, runErr := runAGY(runCtx, request, role, timeout)
	if current, getErr := store.Get(context.Background(), id); getErr == nil && current.Status == StatusCancelled {
		return nil
	}
	hash := sha256.Sum256(output)
	receipt := Receipt{Output: normalizeJSON(output), OutputHash: "sha256:" + hex.EncodeToString(hash[:]), ExitCode: exitCode}
	if runErr == nil {
		if receiptErr := validateStructuredReceipt(output, request.OutputSchema); receiptErr == nil {
			return store.Complete(ctx, id, StatusSucceeded, receipt, "", "")
		} else {
			runErr = receiptErr
		}
	}
	status, code := StatusFailed, "AGY_FAILED"
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		status, code = StatusTimedOut, "TIMEOUT"
	} else if errors.Is(runErr, errInvalidReceipt) {
		code = "INVALID_RECEIPT"
	}
	message := runErr.Error()
	if completeErr := store.Complete(context.Background(), id, status, receipt, code, message); completeErr != nil {
		return errors.Join(runErr, completeErr)
	}
	return runErr
}

func keepAliveAuthority(ctx context.Context, store *Store, taskID string, done <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			_ = store.ExtendTaskAuthority(context.Background(), taskID, 5*time.Minute)
		}
	}
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
	var workspaceBaseline map[string]string
	if request.Role == "implement" {
		cmd.Dir = request.executionDirectory()
		if request.WorkspaceMode == WorkspaceIsolated {
			if err := ensureCleanWorktree(cmd.Dir); err != nil {
				return nil, -1, err
			}
		} else {
			workspaceBaseline, err = captureWorkspaceBaseline(cmd.Dir)
			if err != nil {
				return nil, -1, err
			}
		}
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
				switch msg.StepUpdate.State {
				case "ACTIVE":
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
				case "DONE":
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
	if request.Role == "implement" && (err == nil || request.WorkspaceMode == WorkspaceCurrent) {
		var allowErr error
		if request.WorkspaceMode == WorkspaceIsolated {
			allowErr = validateWorktreeChanges(request.executionDirectory(), request.AllowedFiles)
		} else {
			allowErr = validateWorkspaceChanges(request.executionDirectory(), request.AllowedFiles, workspaceBaseline)
		}
		if allowErr != nil {
			if err != nil {
				err = errors.Join(err, allowErr)
			} else {
				err = allowErr
			}
		}
	}

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
			ExecutionStatus     string `json:"execution_status"`
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
	allowed := map[string]bool{
		"PATH": true, "PATHEXT": true, "SYSTEMROOT": true, "WINDIR": true, "COMSPEC": true,
		"TEMP": true, "TMP": true, "TMPDIR": true, "LANG": true, "LC_ALL": true, "TERM": true,
		"COLORTERM": true, "NO_COLOR": true, "HTTP_PROXY": true, "HTTPS_PROXY": true, "NO_PROXY": true,
		"SSL_CERT_FILE": true, "SSL_CERT_DIR": true,
	}
	environment := make([]string, 0, len(allowed)+4)
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if allowed[strings.ToUpper(key)] {
			environment = append(environment, item)
		}
	}
	environment = append(environment, "HOME="+home, "USERPROFILE="+home)
	if volume := filepath.VolumeName(home); volume != "" {
		environment = append(environment, "HOMEDRIVE="+volume, "HOMEPATH="+strings.TrimPrefix(home, volume))
	}
	return environment
}

func samePath(left, right string) bool {
	left, right = filepath.Clean(left), filepath.Clean(right)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func gitOutput(directory string, args ...string) ([]byte, error) {
	git, err := exec.LookPath("git")
	if err != nil {
		return nil, errors.New("git executable not found")
	}
	cmd := exec.Command(git, args...)
	cmd.Dir = directory
	output, err := cmd.Output()
	if err != nil {
		stderr := ""
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr = string(exitErr.Stderr)
		}
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, bounded(stderr, 512))
	}
	return output, nil
}

func ensureCleanWorktree(directory string) error {
	paths, err := changedWorktreePaths(directory)
	if err != nil {
		return fmt.Errorf("validate isolated worktree %q: %w", directory, err)
	}
	if len(paths) != 0 {
		sample := paths
		if len(sample) > 5 {
			sample = sample[:5]
		}
		more := ""
		if len(paths) > 5 {
			more = fmt.Sprintf(" (+%d more)", len(paths)-5)
		}
		return fmt.Errorf("implement worktree %q must be clean before delegation; found %d uncommitted/untracked file(s): %s%s (clean worktree via git clean/reset/stash or use 'current_workspace' strategy if working directly in the workspace)",
			directory, len(paths), strings.Join(sample, ", "), more)
	}
	return nil
}

func validateRelatedWorktree(workspace, worktree string) error {
	workspaceCommon, err := gitOutput(workspace, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return fmt.Errorf("controller workspace %q is not a git repository: %w", workspace, err)
	}
	worktreeCommon, err := gitOutput(worktree, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return fmt.Errorf("isolated worktree %q is not a git repository: %w", worktree, err)
	}
	if !samePath(strings.TrimSpace(string(workspaceCommon)), strings.TrimSpace(string(worktreeCommon))) {
		return fmt.Errorf("isolated worktree %q does not belong to controller repository %q", worktree, workspace)
	}
	workspaceHead, err := gitOutput(workspace, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("failed to determine controller HEAD in %q: %w", workspace, err)
	}
	worktreeHead, err := gitOutput(worktree, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("failed to determine isolated worktree HEAD in %q: %w", worktree, err)
	}
	if strings.TrimSpace(string(workspaceHead)) != strings.TrimSpace(string(worktreeHead)) {
		return fmt.Errorf("isolated worktree %q HEAD (%s) must start at controller HEAD (%s); sync worktree before delegation",
			worktree, strings.TrimSpace(string(worktreeHead)), strings.TrimSpace(string(workspaceHead)))
	}
	return nil
}

func validateWorktreeChanges(directory string, allowedFiles []string) error {
	changed, err := changedWorktreePaths(directory)
	if err != nil {
		return fmt.Errorf("validate delegated changes in %q: %w", directory, err)
	}
	allowed := make(map[string]struct{}, len(allowedFiles))
	for _, value := range allowedFiles {
		clean, pathErr := canonicalLeasePath(value)
		if pathErr != nil {
			return pathErr
		}
		allowed[clean] = struct{}{}
	}
	var unleased []string
	for _, pathValue := range changed {
		clean, pathErr := canonicalLeasePath(pathValue)
		if pathErr != nil {
			return fmt.Errorf("delegated change has unsafe path %q: %w", pathValue, pathErr)
		}
		if _, ok := allowed[clean]; !ok {
			unleased = append(unleased, clean)
		}
	}
	if len(unleased) > 0 {
		sample := unleased
		if len(sample) > 5 {
			sample = sample[:5]
		}
		more := ""
		if len(unleased) > 5 {
			more = fmt.Sprintf(" (+%d more)", len(unleased)-5)
		}
		return fmt.Errorf("delegated worker modified %d unleased path(s): %s%s (allowed files: %s)",
			len(unleased), strings.Join(sample, ", "), more, strings.Join(allowedFiles, ", "))
	}
	return nil
}

func captureWorkspaceBaseline(directory string) (map[string]string, error) {
	changed, err := changedWorktreePaths(directory)
	if err != nil {
		return nil, fmt.Errorf("capture current workspace baseline: %w", err)
	}
	baseline := make(map[string]string, len(changed))
	for _, value := range changed {
		clean, pathErr := canonicalLeasePath(value)
		if pathErr != nil {
			return nil, pathErr
		}
		fingerprint, fingerprintErr := workspacePathFingerprint(directory, clean)
		if fingerprintErr != nil {
			return nil, fingerprintErr
		}
		baseline[clean] = fingerprint
	}
	return baseline, nil
}

func validateWorkspaceChanges(directory string, allowedFiles []string, baseline map[string]string) error {
	changed, err := changedWorktreePaths(directory)
	if err != nil {
		return fmt.Errorf("validate current workspace changes in %q: %w", directory, err)
	}
	allowed := make(map[string]struct{}, len(allowedFiles))
	for _, value := range allowedFiles {
		clean, pathErr := canonicalLeasePath(value)
		if pathErr != nil {
			return pathErr
		}
		allowed[clean] = struct{}{}
	}
	after := make(map[string]struct{}, len(changed))
	for _, value := range changed {
		clean, pathErr := canonicalLeasePath(value)
		if pathErr != nil {
			return fmt.Errorf("delegated change has unsafe path %q: %w", value, pathErr)
		}
		after[clean] = struct{}{}
	}
	paths := make(map[string]struct{}, len(baseline)+len(after))
	for value := range baseline {
		paths[value] = struct{}{}
	}
	for value := range after {
		paths[value] = struct{}{}
	}
	ordered := make([]string, 0, len(paths))
	for value := range paths {
		ordered = append(ordered, value)
	}
	sort.Strings(ordered)
	var violations []string
	for _, value := range ordered {
		if _, ok := allowed[value]; ok {
			continue
		}
		beforeFingerprint, existedBefore := baseline[value]
		_, existsAfter := after[value]
		if existedBefore != existsAfter {
			violations = append(violations, fmt.Sprintf("unleased path %q was created or deleted", value))
			continue
		}
		if !existedBefore {
			continue
		}
		afterFingerprint, fingerprintErr := workspacePathFingerprint(directory, value)
		if fingerprintErr != nil {
			return fingerprintErr
		}
		if beforeFingerprint != afterFingerprint {
			violations = append(violations, fmt.Sprintf("pre-existing unleased path %q was modified", value))
		}
	}
	if len(violations) > 0 {
		sample := violations
		if len(sample) > 5 {
			sample = sample[:5]
		}
		more := ""
		if len(violations) > 5 {
			more = fmt.Sprintf(" (+%d more)", len(violations)-5)
		}
		return fmt.Errorf("delegated worker violated workspace baseline on %d unleased path(s): %s%s (allowed files: %s)",
			len(violations), strings.Join(sample, "; "), more, strings.Join(allowedFiles, ", "))
	}
	return nil
}

func workspacePathFingerprint(directory, relativePath string) (string, error) {
	clean, err := canonicalLeasePath(relativePath)
	if err != nil {
		return "", err
	}
	target := filepath.Join(directory, filepath.FromSlash(clean))
	info, err := os.Lstat(target)
	if errors.Is(err, os.ErrNotExist) {
		return "missing", nil
	}
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		value, readErr := os.Readlink(target)
		if readErr != nil {
			return "", readErr
		}
		digest := sha256.Sum256([]byte("symlink:" + value))
		return hex.EncodeToString(digest[:]), nil
	}
	if !info.Mode().IsRegular() {
		digest := sha256.Sum256([]byte("mode:" + info.Mode().String()))
		return hex.EncodeToString(digest[:]), nil
	}
	file, err := os.Open(target)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func changedWorktreePaths(directory string) ([]string, error) {
	tracked, err := gitOutput(directory, "diff", "--name-only", "--no-renames", "-z", "HEAD", "--")
	if err != nil {
		return nil, err
	}
	untracked, err := gitOutput(directory, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	combined := append(tracked, untracked...)
	paths := make([]string, 0)
	seen := make(map[string]struct{})
	for _, raw := range bytes.Split(combined, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		value := filepath.ToSlash(string(raw))
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		paths = append(paths, value)
	}
	return paths, nil
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

type receiptSchemaNode struct {
	Type                 string                     `json:"type"`
	Required             []string                   `json:"required"`
	Properties           map[string]json.RawMessage `json:"properties"`
	Items                json.RawMessage            `json:"items"`
	Enum                 []json.RawMessage          `json:"enum"`
	AdditionalProperties *bool                      `json:"additionalProperties"`
}

func validateStructuredReceipt(output, schemaJSON json.RawMessage) error {
	if len(schemaJSON) == 0 {
		return nil
	}
	var envelope struct {
		StructuredOutput json.RawMessage `json:"structured_output"`
	}
	if err := json.Unmarshal(output, &envelope); err != nil {
		return fmt.Errorf("%w: decode AGY result envelope: %v", errInvalidReceipt, err)
	}
	if len(envelope.StructuredOutput) == 0 || bytes.Equal(bytes.TrimSpace(envelope.StructuredOutput), []byte("null")) {
		return fmt.Errorf("%w: structured_output is missing", errInvalidReceipt)
	}
	if err := validateReceiptValue(envelope.StructuredOutput, schemaJSON, "structured_output", true); err != nil {
		return fmt.Errorf("%w: %v", errInvalidReceipt, err)
	}
	return nil
}

func validateReceiptValue(valueJSON, schemaJSON json.RawMessage, field string, required bool) error {
	var schema receiptSchemaNode
	if err := json.Unmarshal(schemaJSON, &schema); err != nil {
		return fmt.Errorf("decode schema for %s: %w", field, err)
	}
	if schema.Type == "" {
		return fmt.Errorf("schema for %s has no supported type", field)
	}
	decoder := json.NewDecoder(bytes.NewReader(valueJSON))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("decode %s: %w", field, err)
	}
	if len(schema.Enum) > 0 && !receiptEnumContains(value, schema.Enum) {
		return fmt.Errorf("%s is outside the allowed enum", field)
	}

	switch schema.Type {
	case "object":
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("%s must be an object", field)
		}
		requiredFields := make(map[string]bool, len(schema.Required))
		for _, name := range schema.Required {
			requiredFields[name] = true
			if _, exists := object[name]; !exists {
				return fmt.Errorf("%s.%s is required", field, name)
			}
		}
		if schema.AdditionalProperties != nil && !*schema.AdditionalProperties {
			for name := range object {
				if _, allowed := schema.Properties[name]; !allowed {
					return fmt.Errorf("%s.%s is not allowed", field, name)
				}
			}
		}
		for name, propertySchema := range schema.Properties {
			property, exists := object[name]
			if !exists {
				continue
			}
			propertyJSON, err := json.Marshal(property)
			if err != nil {
				return fmt.Errorf("encode %s.%s: %w", field, name, err)
			}
			if err := validateReceiptValue(propertyJSON, propertySchema, field+"."+name, requiredFields[name]); err != nil {
				return err
			}
		}
	case "array":
		items, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%s must be an array", field)
		}
		if len(schema.Items) > 0 {
			for index, item := range items {
				itemJSON, err := json.Marshal(item)
				if err != nil {
					return fmt.Errorf("encode %s[%d]: %w", field, index, err)
				}
				if err := validateReceiptValue(itemJSON, schema.Items, fmt.Sprintf("%s[%d]", field, index), false); err != nil {
					return err
				}
			}
		}
	case "string":
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s must be a string", field)
		}
		if required && strings.TrimSpace(text) == "" {
			return fmt.Errorf("%s must not be empty", field)
		}
	case "number":
		if _, ok := value.(json.Number); !ok {
			return fmt.Errorf("%s must be a number", field)
		}
	case "integer":
		number, ok := value.(json.Number)
		if !ok || strings.ContainsAny(string(number), ".eE") {
			return fmt.Errorf("%s must be an integer", field)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s must be a boolean", field)
		}
	case "null":
		if value != nil {
			return fmt.Errorf("%s must be null", field)
		}
	default:
		return fmt.Errorf("schema type %q for %s is unsupported", schema.Type, field)
	}
	return nil
}

func receiptEnumContains(value any, allowed []json.RawMessage) bool {
	encoded, err := json.Marshal(value)
	if err != nil {
		return false
	}
	for _, candidate := range allowed {
		var compact bytes.Buffer
		if json.Compact(&compact, candidate) == nil && bytes.Equal(encoded, compact.Bytes()) {
			return true
		}
	}
	return false
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

package app

import (
	"fmt"
	"strings"
)

func runCompletion(args []string) error {
	if len(args) == 0 || isHelp(args[0]) {
		fmt.Println("Usage: cortex-ia completion <shell>")
		fmt.Println("\nSupported shells:")
		fmt.Println("  bash    Generate bash completion script")
		fmt.Println("  zsh     Generate zsh completion script")
		fmt.Println("  fish    Generate fish completion script")
		fmt.Println("\nInstallation examples:")
		fmt.Println("  Bash:   eval \"$(cortex-ia completion bash)\"")
		fmt.Println("  Zsh:    source <(cortex-ia completion zsh)")
		fmt.Println("  Fish:   cortex-ia completion fish | source")
		return nil
	}

	shell := strings.ToLower(args[0])
	switch shell {
	case "bash":
		fmt.Print(bashCompletionScript)
		return nil
	case "zsh":
		fmt.Print(zshCompletionScript)
		return nil
	case "fish":
		fmt.Print(fishCompletionScript)
		return nil
	default:
		return fmt.Errorf("unsupported shell %q (supported: bash, zsh, fish)", shell)
	}
}

const bashCompletionScript = `_cortex_ia_completions() {
    local cur prev words cword
    _init_completion -n : || return

    local top_commands="install sync snapshot work worktree board ledger ui openspec web doc diagram report stats doctor rollback recover uninstall update completion version help mcp model"

    if [ "$cword" -eq 1 ]; then
        COMPREPLY=($(compgen -W "$top_commands" -- "$cur"))
        return 0
    fi

    local cmd="${words[1]}"
    case "$cmd" in
        work)
            local work_subs="create revise archive list status approvals fingerprint claim renew controller-renew transition approve retry lease reserve lease-renew release release-all verify-lease review-refresh decompose recover reconcile degrade"
            if [ "$cword" -eq 2 ]; then
                COMPREPLY=($(compgen -W "$work_subs" -- "$cur"))
            fi
            ;;
        board)
            local board_subs="create list status archive unarchive delete serve"
            if [ "$cword" -eq 2 ]; then
                COMPREPLY=($(compgen -W "$board_subs" -- "$cur"))
            fi
            ;;
        mcp)
            local mcp_subs="add list remove"
            if [ "$cword" -eq 2 ]; then
                COMPREPLY=($(compgen -W "$mcp_subs" -- "$cur"))
            fi
            ;;
        model)
            local model_subs="list get set unset doctor catalog"
            if [ "$cword" -eq 2 ]; then
                COMPREPLY=($(compgen -W "$model_subs" -- "$cur"))
            fi
            ;;
        ledger)
            local ledger_subs="fact progress status"
            if [ "$cword" -eq 2 ]; then
                COMPREPLY=($(compgen -W "$ledger_subs" -- "$cur"))
            fi
            ;;
        openspec)
            local openspec_subs="validate list status archive new"
            if [ "$cword" -eq 2 ]; then
                COMPREPLY=($(compgen -W "$openspec_subs" -- "$cur"))
            fi
            ;;
        completion)
            if [ "$cword" -eq 2 ]; then
                COMPREPLY=($(compgen -W "bash zsh fish" -- "$cur"))
            fi
            ;;
        *)
            ;;
    esac
}
complete -F _cortex_ia_completions cortex-ia
`

const zshCompletionScript = `#compdef cortex-ia

_cortex_ia() {
    local -a commands
    commands=(
        'install:Install native OpenCode assets and configure MCP'
        'sync:Synchronize installed assets with active repository'
        'snapshot:Read and verify bounded Cortex snapshot'
        'work:Define, revise, claim, or inspect local task DAG'
        'worktree:Inspect authoritative Git worktrees'
        'board:Group task DAGs into local task boards'
        'ledger:Inspect or update the Dual Ledger (facts + progress)'
        'ui:Print a bounded read-only TUI snapshot'
        'openspec:Manage the OpenSpec SDD workspace'
        'web:Launch local Cortex-IA web dashboard in browser'
        'doc:Convert office/PDF docs to Markdown or inspect metadata'
        'diagram:Validate, render, compare, or trace system diagrams'
        'report:Report errors or manage reporting configuration'
        'stats:Print bounded read-only usage statistics'
        'doctor:Assess installation health'
        'rollback:Restore a previous backup'
        'recover:Manage and restore pending recovery journals'
        'uninstall:Remove the accredited installation'
        'update:Check for and install latest release'
        'mcp:Manage MCP preset and custom servers'
        'model:Configure and inspect agent models'
        'completion:Generate shell autocompletion script'
        'version:Show version'
        'help:Show help'
    )

    if (( CURRENT == 2 )); then
        _describe -t commands 'cortex-ia command' commands
        return
    fi

    case "$words[2]" in
        work)
            local -a work_commands
            work_commands=(
                'create:Define a new task'
                'list:List tasks'
                'status:Inspect task details'
                'claim:Acquire task execution claim'
                'transition:Move task state (in_progress, in_review, blocked)'
                'approve:Approve completed task with evidence'
                'lease:Reserve a file path'
                'release:Release file reservation'
                'recover:Recover expired claims and leases'
                'reconcile:Force-release orphaned claims'
                'decompose:Split task into atomic sub-tasks'
                'degrade:Degrade stale ready/blocked tasks to backlog'
            )
            _describe -t work_commands 'work subcommand' work_commands
            ;;
        board)
            local -a board_commands
            board_commands=(
                'create:Create a new task board'
                'list:List all boards'
                'status:Inspect board snapshot'
                'archive:Archive completed board'
                'unarchive:Restore archived board'
                'delete:Delete archived board'
                'serve:Serve web operations dashboard'
            )
            _describe -t board_commands 'board subcommand' board_commands
            ;;
        mcp)
            local -a mcp_commands
            mcp_commands=(
                'add:Register a managed or custom MCP server'
                'list:List registered MCP servers'
                'remove:Deregister an MCP server'
            )
            _describe -t mcp_commands 'mcp subcommand' mcp_commands
            ;;
        model)
            local -a model_commands
            model_commands=(
                'list:List agents with active models'
                'get:Report an agent model'
                'set:Assign an agent model'
                'unset:Remove an agent model assignment'
                'doctor:Diagnose agent model configuration'
                'catalog:List selectable models'
            )
            _describe -t model_commands 'model subcommand' model_commands
            ;;
        completion)
            local -a shells
            shells=('bash:Bash shell' 'zsh:Zsh shell' 'fish:Fish shell')
            _describe -t shells 'shell' shells
            ;;
    esac
}

_cortex_ia "$@"
`

const fishCompletionScript = `# Fish shell completion for cortex-ia

# Top-level commands
complete -c cortex-ia -n "__fish_use_subcommand" -a "install" -d "Install native OpenCode assets"
complete -c cortex-ia -n "__fish_use_subcommand" -a "sync" -d "Synchronize installed assets"
complete -c cortex-ia -n "__fish_use_subcommand" -a "snapshot" -d "Read and verify Cortex snapshot"
complete -c cortex-ia -n "__fish_use_subcommand" -a "work" -d "Define, claim, or advance task DAG"
complete -c cortex-ia -n "__fish_use_subcommand" -a "board" -d "Manage local task boards"
complete -c cortex-ia -n "__fish_use_subcommand" -a "ledger" -d "Dual Ledger facts and progress"
complete -c cortex-ia -n "__fish_use_subcommand" -a "openspec" -d "Manage OpenSpec workspace"
complete -c cortex-ia -n "__fish_use_subcommand" -a "web" -d "Launch web dashboard"
complete -c cortex-ia -n "__fish_use_subcommand" -a "mcp" -d "Manage MCP servers"
complete -c cortex-ia -n "__fish_use_subcommand" -a "model" -d "Manage agent models"
complete -c cortex-ia -n "__fish_use_subcommand" -a "doctor" -d "Assess installation health"
complete -c cortex-ia -n "__fish_use_subcommand" -a "completion" -d "Generate shell completion script"
complete -c cortex-ia -n "__fish_use_subcommand" -a "version" -d "Show version"
complete -c cortex-ia -n "__fish_use_subcommand" -a "help" -d "Show help"

# Subcommands: work
complete -c cortex-ia -n "__fish_seen_subcommand_from work" -a "create list status claim transition approve lease release recover reconcile decompose degrade"

# Subcommands: board
complete -c cortex-ia -n "__fish_seen_subcommand_from board" -a "create list status archive unarchive delete serve"

# Subcommands: mcp
complete -c cortex-ia -n "__fish_seen_subcommand_from mcp" -a "add list remove"

# Subcommands: model
complete -c cortex-ia -n "__fish_seen_subcommand_from model" -a "list get set unset doctor catalog"

# Subcommands: completion
complete -c cortex-ia -n "__fish_seen_subcommand_from completion" -a "bash zsh fish"
`

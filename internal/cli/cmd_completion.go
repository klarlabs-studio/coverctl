package cli

import (
	"fmt"
	"io"
)

func runCompletion(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "Usage: coverctl completion <bash|zsh|fish>")
		return 2
	}

	switch args[0] {
	case "bash":
		fmt.Fprintln(stdout, bashCompletion)
	case "zsh":
		fmt.Fprintln(stdout, zshCompletion)
	case "fish":
		fmt.Fprintln(stdout, fishCompletion)
	default:
		fmt.Fprintf(stderr, "Unknown shell: %s\nSupported: bash, zsh, fish\n", args[0])
		return 2
	}
	return 0
}

const bashCompletion = `# coverctl bash completion
_coverctl() {
    local cur prev commands global_flags
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    commands="check run watch init detect report badge trend record suggest debt ignore mcp survey help version completion c r w i"
    global_flags="-q --quiet --no-color --ci --debug"

    if [[ ${COMP_CWORD} -eq 1 ]]; then
        COMPREPLY=( $(compgen -W "${commands} ${global_flags}" -- ${cur}) )
        return 0
    fi

    case "${prev}" in
        -c|--config)
            COMPREPLY=( $(compgen -f -X '!*.yaml' -- ${cur}) )
            return 0
            ;;
        -p|--profile)
            COMPREPLY=( $(compgen -f -X '!*.out' -- ${cur}) )
            return 0
            ;;
        -o|--output)
            COMPREPLY=( $(compgen -W "text json html" -- ${cur}) )
            return 0
            ;;
        --strategy)
            COMPREPLY=( $(compgen -W "current aggressive conservative" -- ${cur}) )
            return 0
            ;;
        --style)
            COMPREPLY=( $(compgen -W "flat flat-square" -- ${cur}) )
            return 0
            ;;
        completion)
            COMPREPLY=( $(compgen -W "bash zsh fish" -- ${cur}) )
            return 0
            ;;
        mcp)
            COMPREPLY=( $(compgen -W "serve doctor" -- ${cur}) )
            return 0
            ;;
    esac

    COMPREPLY=( $(compgen -W "-c --config -p --profile -d --domain -o --output -f --force -h --help -q --quiet --no-color --ci --uncovered --diff --merge --show-delta --history --fail-under --ratchet --validate --tags --race --short -v --run --timeout --max-runtime --test-arg" -- ${cur}) )
}
complete -F _coverctl coverctl`

const zshCompletion = `#compdef coverctl

_coverctl() {
    local -a commands
    commands=(
        'check:Run coverage and enforce policy'
        'c:Run coverage and enforce policy (alias)'
        'run:Run coverage only, produce artifacts'
        'r:Run coverage only (alias)'
        'watch:Watch for file changes and re-run coverage'
        'w:Watch for file changes (alias)'
        'init:Interactive setup wizard'
        'i:Interactive setup wizard (alias)'
        'detect:Autodetect domains and write config'
        'report:Analyze an existing profile'
        'badge:Generate an SVG coverage badge'
        'trend:Show coverage trends over time'
        'record:Record current coverage to history'
        'suggest:Suggest optimal coverage thresholds'
        'debt:Show coverage debt report'
        'ignore:Show configured excludes and ignore advice'
        'mcp:MCP server for AI agents'
        'help:Show help for a command'
        'version:Show version information'
        'completion:Generate shell completion scripts'
    )

    _arguments -C \
        '-q[Suppress non-essential output]' \
        '--quiet[Suppress non-essential output]' \
        '--no-color[Disable colored output]' \
        '--ci[CI mode: quiet + GitHub Actions annotations]' \
        '1: :->command' \
        '*: :->args'

    case $state in
        command)
            _describe 'command' commands
            ;;
        args)
            case $words[2] in
                check|c|run|r|watch|w|report|badge|trend|record|suggest|debt|ignore|init|i|detect)
                    _arguments \
                        '-c[Config file path]:file:_files -g "*.yaml"' \
                        '--config[Config file path]:file:_files -g "*.yaml"' \
                        '-p[Coverage profile path]:file:_files -g "*.out"' \
                        '--profile[Coverage profile path]:file:_files -g "*.out"' \
                        '--from-profile[Use existing coverage profile instead of running tests]' \
                        '-d[Filter to domain]:domain:' \
                        '--domain[Filter to domain]:domain:' \
                        '-o[Output format]:format:(text json html)' \
                        '--output[Output format]:format:(text json html)' \
                        '-f[Force overwrite]' \
                        '--force[Force overwrite]' \
                        '--uncovered[Show only files with 0% coverage]' \
                        '--diff[Show coverage for changed files]:ref:' \
                        '--merge[Merge additional profile]:file:_files -g "*.out"' \
                        '--show-delta[Show coverage change from previous run]' \
                        '--history[History file path]:file:_files -g "*.json"' \
                        '--fail-under[Fail if coverage below threshold]:percent:' \
                        '--ratchet[Fail if coverage decreases]' \
                        '--validate[Validate config without running tests]' \
                        '--tags[Build tags]:tags:' \
                        '--race[Enable race detector]' \
                        '--short[Skip long-running tests]' \
                        '-v[Verbose test output]' \
                        '--run[Run tests matching pattern]:pattern:' \
                        '--test-run[Run tests matching pattern]:pattern:' \
                        '--timeout[Test timeout]:duration:' \
                        '--test-arg[Additional test argument]:arg:' \
                        '--language[Override language detection]:lang:(python javascript typescript java rust go csharp cpp php ruby swift dart scala elixir shell)'
                    ;;
                completion)
                    _arguments '1:shell:(bash zsh fish)'
                    ;;
                mcp)
                    _arguments '1:subcommand:(serve)'
                    ;;
            esac
            ;;
    esac
}

_coverctl "$@"`

const fishCompletion = `# coverctl fish completion
complete -c coverctl -f

# Global flags
complete -c coverctl -s q -l quiet -d "Suppress non-essential output"
complete -c coverctl -l no-color -d "Disable colored output"
complete -c coverctl -l ci -d "CI mode: quiet + GitHub Actions annotations"

# Commands
complete -c coverctl -n "__fish_use_subcommand" -a "check" -d "Run coverage and enforce policy"
complete -c coverctl -n "__fish_use_subcommand" -a "c" -d "Run coverage and enforce policy (alias)"
complete -c coverctl -n "__fish_use_subcommand" -a "run" -d "Run coverage only, produce artifacts"
complete -c coverctl -n "__fish_use_subcommand" -a "r" -d "Run coverage only (alias)"
complete -c coverctl -n "__fish_use_subcommand" -a "watch" -d "Watch for file changes and re-run coverage"
complete -c coverctl -n "__fish_use_subcommand" -a "w" -d "Watch for file changes (alias)"
complete -c coverctl -n "__fish_use_subcommand" -a "init" -d "Interactive setup wizard"
complete -c coverctl -n "__fish_use_subcommand" -a "i" -d "Interactive setup wizard (alias)"
complete -c coverctl -n "__fish_use_subcommand" -a "detect" -d "Autodetect domains and write config"
complete -c coverctl -n "__fish_use_subcommand" -a "report" -d "Analyze an existing profile"
complete -c coverctl -n "__fish_use_subcommand" -a "badge" -d "Generate an SVG coverage badge"
complete -c coverctl -n "__fish_use_subcommand" -a "trend" -d "Show coverage trends over time"
complete -c coverctl -n "__fish_use_subcommand" -a "record" -d "Record current coverage to history"
complete -c coverctl -n "__fish_use_subcommand" -a "suggest" -d "Suggest optimal coverage thresholds"
complete -c coverctl -n "__fish_use_subcommand" -a "debt" -d "Show coverage debt report"
complete -c coverctl -n "__fish_use_subcommand" -a "ignore" -d "Show configured excludes"
complete -c coverctl -n "__fish_use_subcommand" -a "mcp" -d "MCP server for AI agents"
complete -c coverctl -n "__fish_use_subcommand" -a "help" -d "Show help for a command"
complete -c coverctl -n "__fish_use_subcommand" -a "version" -d "Show version information"
complete -c coverctl -n "__fish_use_subcommand" -a "completion" -d "Generate shell completion"

# Flags for all commands
complete -c coverctl -s c -l config -d "Config file path" -r -F
complete -c coverctl -s p -l profile -d "Coverage profile path" -r -F
complete -c coverctl -l from-profile -d "Use existing coverage profile instead of running tests"
complete -c coverctl -s d -l domain -d "Filter to specific domain" -r
complete -c coverctl -s o -l output -d "Output format" -r -a "text json html"
complete -c coverctl -s f -l force -d "Force overwrite"
complete -c coverctl -s h -l help -d "Show help"
complete -c coverctl -l uncovered -d "Show only files with 0% coverage"
complete -c coverctl -l diff -d "Show coverage for changed files" -r
complete -c coverctl -l merge -d "Merge additional coverage profile" -r -F
complete -c coverctl -l show-delta -d "Show coverage change from previous run"
complete -c coverctl -l history -d "History file path" -r -F
complete -c coverctl -l fail-under -d "Fail if coverage below threshold" -r
complete -c coverctl -l ratchet -d "Fail if coverage decreases"
complete -c coverctl -l validate -d "Validate config without running tests"
complete -c coverctl -l tags -d "Build tags (e.g., integration,e2e)" -r
complete -c coverctl -l race -d "Enable race detector"
complete -c coverctl -l short -d "Skip long-running tests"
complete -c coverctl -s v -d "Verbose test output"
complete -c coverctl -l run -d "Run tests matching pattern" -r
complete -c coverctl -l test-run -d "Run tests matching pattern" -r
complete -c coverctl -l timeout -d "Test timeout (e.g., 10m, 1h)" -r
complete -c coverctl -l test-arg -d "Additional argument passed to go test" -r
complete -c coverctl -l language -d "Override language detection" -r -a "python javascript typescript java rust go csharp cpp php ruby swift dart scala elixir shell"

# Completion subcommand
complete -c coverctl -n "__fish_seen_subcommand_from completion" -a "bash zsh fish"

# MCP subcommand
complete -c coverctl -n "__fish_seen_subcommand_from mcp" -a "serve" -d "Start the MCP server"`

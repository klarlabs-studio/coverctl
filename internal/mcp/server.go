package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/felixgeelhaar/coverctl/internal/application"
	"github.com/felixgeelhaar/coverctl/internal/infrastructure/config"
	"github.com/felixgeelhaar/coverctl/internal/infrastructure/history"
	"github.com/felixgeelhaar/coverctl/internal/pathutil"
	"github.com/felixgeelhaar/mcp-go"
)

// Server wraps the application service with MCP protocol handling.
type Server struct {
	svc            Service
	config         Config
	server         *mcp.Server
	prCommentLimit *rateLimiter
	telemetry      Telemetry // nil = NoopTelemetry (opt-in via config)
}

// New creates a new MCP server wrapping the given service.
func New(svc Service, cfg Config, version string) *Server {
	// Apply defaults
	if cfg.ConfigPath == "" {
		cfg.ConfigPath = DefaultConfig().ConfigPath
	}
	if cfg.HistoryPath == "" {
		cfg.HistoryPath = DefaultConfig().HistoryPath
	}
	if cfg.ProfilePath == "" {
		cfg.ProfilePath = DefaultConfig().ProfilePath
	}
	if cfg.Mode == "" {
		cfg.Mode = ModeAgent
	}
	if version == "" {
		version = "dev"
	}

	// Only auto-detect config if using default path and it doesn't exist
	// This preserves explicit custom paths specified by the user
	defaultPath := DefaultConfig().ConfigPath
	if cfg.ConfigPath == defaultPath {
		if _, err := os.Stat(cfg.ConfigPath); os.IsNotExist(err) {
			if foundPath, findErr := config.FindConfigFrom(""); findErr == nil {
				cfg.ConfigPath = foundPath
			}
		}
	}

	s := &Server{
		svc:            svc,
		config:         cfg,
		prCommentLimit: newRateLimiter(),
		telemetry:      NoopTelemetry{},
	}

	// Create MCP server with capabilities
	s.server = mcp.NewServer(mcp.ServerInfo{
		Name:    "coverctl",
		Version: version,
		Capabilities: mcp.Capabilities{
			Tools:     true,
			Resources: true,
		},
	})

	// Register tools and resources
	s.registerTools()
	s.registerResources()

	return s
}

// Run starts the MCP server and blocks until the context is canceled.
func (s *Server) Run(ctx context.Context) error {
	return mcp.ServeStdio(ctx, s.server)
}

// registerTools adds tool handlers to the server, gated by s.config.Mode.
//
// Agent mode (default) advertises only the three agent-loop tools (check,
// suggest, debt) so coding agents have a small, reliable selection
// surface. CI mode adds setup, dashboarding, and CI/automation tools
// (init, report, record, badge, compare, pr-comment) for non-agent
// callers.
//
// The handler for every tool is registered identically — the only thing
// mode controls is whether the tool is *advertised* to the client. Tools
// not advertised are not callable through this server instance.
func (s *Server) registerTools() {
	agent := s.config.Mode == ModeAgent

	s.server.Tool("check").
		Description("Run the project's test suite with coverage and enforce per-domain policy thresholds defined in .coverctl.yaml. Auto-detects the language and invokes the appropriate test runner (go test, pytest, npm test, mvn, gradle, cargo, dotnet, etc.). Returns per-domain pass/fail, file-level coverage, and warnings. Exit-equivalent: passed=true on success, passed=false on policy violation or runner error.").
		Handler(s.handleCheck)

	s.server.Tool("suggest").
		Description("Analyze current coverage and suggest threshold values for each domain. Strategies: 'current' (set thresholds slightly below current observed coverage to lock in the status quo), 'aggressive' (set targets above current to push improvement), 'conservative' (small incremental gains). Use writeConfig=true to apply suggestions; coverctl backs up the existing file first.").
		Handler(s.handleSuggest)

	s.server.Tool("debt").
		Description("Compute coverage debt: the gap between current coverage and required thresholds, ranked per domain and per file. Returns a health score (0-100) and the items contributing the most debt. Use this to direct test-writing effort to the highest-impact gaps.").
		Handler(s.handleDebt)

	if agent {
		return
	}

	s.server.Tool("init").
		Description("Initialize coverctl in the current project. Auto-detects the project's language (Go, Python, TypeScript/JavaScript, Java, Rust, C#, C/C++, PHP, Ruby, Swift, Dart, Scala, Elixir, or Shell), proposes domain boundaries from the directory layout, and writes .coverctl.yaml with default thresholds. Call once per project.").
		Handler(s.handleInit)

	s.server.Tool("report").
		Description("Analyze an existing coverage profile without re-running tests. Supports Go cover profiles, LCOV (info), Cobertura (XML), and JaCoCo (XML); format auto-detected from file content. Use when a profile is already on disk from a prior CI run or a separate test invocation.").
		Handler(s.handleReport)

	s.server.Tool("record").
		Description("Append the current coverage snapshot to the project's history file for trend tracking. Captures commit/branch metadata. Run after 'check' (or pass run=true to run coverage first) so subsequent 'compare' and trend resource calls have data points.").
		Handler(s.handleRecord)

	s.server.Tool("badge").
		Description("Generate an SVG coverage badge suitable for embedding in a README or dashboard. Returns the badge SVG and the overall coverage percent; if 'output' is set, also writes the SVG to that path.").
		Handler(s.handleBadge)

	s.server.Tool("compare").
		Description("Diff coverage between two profiles (base vs head). Returns overall delta, files whose coverage improved, files whose coverage regressed, and domain-level deltas. Use to evaluate whether a code change improved or worsened coverage before committing.").
		Handler(s.handleCompare)

	s.server.Tool("pr-comment").
		Description("Post the coverage report as a comment on a pull/merge request. Supports GitHub, GitLab, and Bitbucket; provider is auto-detected from environment variables or set explicitly. Reuses an existing coverctl comment when present (idempotent). Requires the appropriate provider token in the environment.").
		Handler(s.handlePRComment)
}

// registerResources adds all resource handlers to the server.
func (s *Server) registerResources() {
	// Debt resource
	s.server.Resource("coverctl://debt").
		Name("Coverage Debt").
		Description("Shows coverage debt - gap between current and required coverage thresholds").
		MimeType("application/json").
		Handler(s.handleDebtResource)

	// Trend resource
	s.server.Resource("coverctl://trend").
		Name("Coverage Trend").
		Description("Shows coverage trends over time from recorded history").
		MimeType("application/json").
		Handler(s.handleTrendResource)

	// Suggest resource
	s.server.Resource("coverctl://suggest").
		Name("Threshold Suggestions").
		Description("Suggests optimal coverage thresholds based on current coverage").
		MimeType("application/json").
		Handler(s.handleSuggestResource)

	// Config resource
	s.server.Resource("coverctl://config").
		Name("Current Configuration").
		Description("Returns current or auto-detected coverctl configuration").
		MimeType("application/json").
		Handler(s.handleConfigResource)
}

// Tool handlers

func (s *Server) handleCheck(ctx context.Context, input CheckInput) (map[string]any, error) {
	defer traceTool("check")()
	start := time.Now()
	defer func() {
		// telemetry recorded by handleCheckEnd
	}()

	if err := validateScopedInputs(
		namedPath{"configPath", input.ConfigPath},
		namedPath{"profile", input.Profile},
	); err != nil {
		s.telemetry.RecordToolCall("check", time.Since(start), err, true)
		return rejectionResponse(err), nil
	}
	if err := SanitizeBuildFlagsInput(input.Tags, input.Run, input.Timeout, input.TestArgs); err != nil {
		s.telemetry.RecordToolCall("check", time.Since(start), err, true)
		return rejectionResponse(err), nil
	}

	opts := application.CheckOptions{
		ConfigPath:     s.resolveConfigPath(input.ConfigPath),
		Profile:        coalesce(input.Profile, s.config.ProfilePath),
		Output:         application.OutputJSON,
		FromProfile:    input.FromProfile,
		Domains:        input.Domains,
		FailUnder:      input.FailUnder,
		Ratchet:        input.Ratchet,
		Incremental:    input.Incremental,
		IncrementalRef: input.IncrementalRef,
		BuildFlags: application.BuildFlags{
			Tags:     input.Tags,
			Race:     input.Race,
			Short:    input.Short,
			Verbose:  input.Verbose,
			Run:      input.Run,
			Timeout:  input.Timeout,
			TestArgs: input.TestArgs,
		},
	}

	// Add history store if ratchet is enabled
	if input.Ratchet {
		opts.HistoryStore = &history.FileStore{Path: s.config.HistoryPath}
	}

	result, err := s.svc.CheckResult(ctx, opts)
	s.telemetry.RecordToolCall("check", time.Since(start), err, false)

	if classified, ok := classifyRuntimeError(err); ok {
		return classified, nil
	}

	v := resolveVerbosity(input.Verbosity)
	domains, domainCursor := applyDomainBudget(result.Domains, v)
	files, fileCursor := applyFileBudget(result.Files, v)
	output := map[string]any{
		"passed":   result.Passed,
		"summary":  sanitizeOutputString(generateSummary(result)),
		"domains":  sanitizeDomainResults(domains),
		"files":    sanitizeFileResults(files),
		"warnings": sanitizeWarnings(result.Warnings),
	}
	if domainCursor != "" {
		output["domainsNextCursor"] = domainCursor
	}
	if fileCursor != "" {
		output["filesNextCursor"] = fileCursor
	}

	if err != nil {
		output["passed"] = false
		output["error"] = sanitizeOutputString(err.Error())
	}

	if err == nil && result.Passed {
		s.telemetry.RecordActivationStep("check_passed", repoFingerprint())
	}

	return output, nil
}

func (s *Server) handleReport(ctx context.Context, input ReportInput) (map[string]any, error) {
	defer traceTool("report")()
	if err := validateScopedInputs(
		namedPath{"configPath", input.ConfigPath},
		namedPath{"profile", input.Profile},
	); err != nil {
		return rejectionResponse(err), nil
	}

	opts := application.ReportOptions{
		ConfigPath:    s.resolveConfigPath(input.ConfigPath),
		Profile:       coalesce(input.Profile, s.config.ProfilePath),
		Output:        application.OutputJSON,
		Domains:       input.Domains,
		ShowUncovered: input.ShowUncovered,
		DiffRef:       input.DiffRef,
	}

	result, err := s.svc.ReportResult(ctx, opts)

	if classified, ok := classifyRuntimeError(err); ok {
		return classified, nil
	}

	v := resolveVerbosity(input.Verbosity)
	domains, domainCursor := applyDomainBudget(result.Domains, v)
	files, fileCursor := applyFileBudget(result.Files, v)
	output := map[string]any{
		"passed":   result.Passed,
		"summary":  sanitizeOutputString(generateSummary(result)),
		"domains":  sanitizeDomainResults(domains),
		"files":    sanitizeFileResults(files),
		"warnings": sanitizeWarnings(result.Warnings),
	}
	if domainCursor != "" {
		output["domainsNextCursor"] = domainCursor
	}
	if fileCursor != "" {
		output["filesNextCursor"] = fileCursor
	}

	if err != nil {
		output["passed"] = false
		output["error"] = sanitizeOutputString(err.Error())
	}

	return output, nil
}

type recordWarner interface {
	RecordWithWarnings(ctx context.Context, opts application.RecordOptions, store application.HistoryStore) (application.RecordResult, error)
}

func (s *Server) handleRecord(ctx context.Context, input RecordInput) (map[string]any, error) {
	defer traceTool("record")()
	if err := validateScopedInputs(
		namedPath{"configPath", input.ConfigPath},
		namedPath{"profile", input.Profile},
		namedPath{"historyPath", input.HistoryPath},
	); err != nil {
		return rejectionResponse(err), nil
	}
	if err := SanitizeBuildFlagsInput(input.Tags, input.TestRun, input.Timeout, input.TestArgs); err != nil {
		return rejectionResponse(err), nil
	}

	opts := application.RecordOptions{
		ConfigPath:  s.resolveConfigPath(input.ConfigPath),
		ProfilePath: coalesce(input.Profile, s.config.ProfilePath),
		HistoryPath: coalesce(input.HistoryPath, s.config.HistoryPath),
		Commit:      input.Commit,
		Branch:      input.Branch,
		Run:         input.Run,
		Domains:     input.Domains,
		BuildFlags: application.BuildFlags{
			Tags:     input.Tags,
			Race:     input.Race,
			Short:    input.Short,
			Verbose:  input.Verbose,
			Run:      input.TestRun,
			Timeout:  input.Timeout,
			TestArgs: input.TestArgs,
		},
		Language: application.Language(input.Language),
	}

	store := &history.FileStore{Path: opts.HistoryPath}

	var recordResult application.RecordResult
	var err error
	if warnSvc, ok := s.svc.(recordWarner); ok {
		recordResult, err = warnSvc.RecordWithWarnings(ctx, opts, store)
	} else {
		err = s.svc.Record(ctx, opts, store)
	}

	output := map[string]any{
		"passed": err == nil,
	}

	if err != nil {
		output["error"] = err.Error()
		output["summary"] = "Failed to record coverage"
	} else {
		output["summary"] = "Coverage recorded to history"
		if len(recordResult.Warnings) > 0 {
			output["warnings"] = recordResult.Warnings
		}
	}

	return output, nil
}

func (s *Server) handleInit(ctx context.Context, input InitInput) (map[string]any, error) {
	defer traceTool("init")()
	if err := validateScopedInputs(namedPath{"configPath", input.ConfigPath}); err != nil {
		return rejectionResponse(err), nil
	}

	// For init, use coalesce (not resolveConfigPath) since we're creating a new file
	configPath := coalesce(input.ConfigPath, s.config.ConfigPath)

	// Check if config already exists
	if !input.Force {
		if _, err := os.Stat(configPath); err == nil {
			return errorResponse(
				OpCodeConfigExists,
				"Config file already exists",
				fmt.Errorf("config file %s already exists (use force: true to overwrite)", configPath),
				"Pass force: true to overwrite the existing config, or call init from a path where .coverctl.yaml does not yet exist.",
			), nil
		}
	}

	// Auto-detect project structure
	cfg, err := s.svc.Detect(ctx, application.DetectOptions{})
	if err != nil {
		return errorResponse(
			OpCodeDetectFailed,
			"Failed to detect project structure",
			err,
			"Ensure the working directory contains a recognized language marker (go.mod, pyproject.toml, package.json, Cargo.toml, pom.xml, ...). For mixed or empty repos, pass language explicitly.",
		), nil
	}

	// Validate and clean the path
	cleanPath, err := pathutil.ValidatePath(configPath)
	if err != nil {
		return errorResponse(
			OpCodeInvalidPath,
			"Invalid config path",
			fmt.Errorf("invalid config path: %v", err),
			"Use a path inside the current working directory. Out-of-tree paths are rejected.",
		), nil
	}

	// Write the config file
	file, err := os.Create(cleanPath) // #nosec G304 - path is validated above
	if err != nil {
		return errorResponse(
			OpCodeFileWrite,
			"Failed to create config file",
			err,
			"Check write permissions on the target path; run init from the repo root.",
		), nil
	}
	defer file.Close()

	if err := config.Write(file, cfg); err != nil {
		return errorResponse(
			OpCodeFileWrite,
			"Failed to write config file",
			err,
			"Check disk space and write permissions on the target path.",
		), nil
	}

	// Build summary of what was detected
	domainCount := len(cfg.Policy.Domains)
	domainNames := make([]string, 0, domainCount)
	for _, d := range cfg.Policy.Domains {
		domainNames = append(domainNames, d.Name)
	}

	s.telemetry.RecordActivationStep("init_completed", repoFingerprint())

	return map[string]any{
		"passed":      true,
		"summary":     fmt.Sprintf("Created %s with %d domains", configPath, domainCount),
		"configPath":  configPath,
		"domains":     domainNames,
		"defaultMin":  cfg.Policy.DefaultMin,
		"domainCount": domainCount,
	}, nil
}

// Resource handlers

func (s *Server) handleDebtResource(ctx context.Context, uri string, params map[string]string) (*mcp.ResourceContent, error) {
	result, err := s.svc.Debt(ctx, application.DebtOptions{
		ConfigPath:  s.config.ConfigPath,
		ProfilePath: s.config.ProfilePath,
		Output:      application.OutputJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to calculate debt: %w", err)
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal debt result: %w", err)
	}

	return &mcp.ResourceContent{
		URI:      uri,
		MimeType: "application/json",
		Text:     string(data),
	}, nil
}

func (s *Server) handleTrendResource(ctx context.Context, uri string, params map[string]string) (*mcp.ResourceContent, error) {
	store := &history.FileStore{Path: s.config.HistoryPath}

	result, err := s.svc.Trend(ctx, application.TrendOptions{
		ConfigPath:  s.config.ConfigPath,
		ProfilePath: s.config.ProfilePath,
		HistoryPath: s.config.HistoryPath,
		Output:      application.OutputJSON,
	}, store)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate trend: %w", err)
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal trend result: %w", err)
	}

	return &mcp.ResourceContent{
		URI:      uri,
		MimeType: "application/json",
		Text:     string(data),
	}, nil
}

func (s *Server) handleSuggestResource(ctx context.Context, uri string, params map[string]string) (*mcp.ResourceContent, error) {
	result, err := s.svc.Suggest(ctx, application.SuggestOptions{
		ConfigPath:  s.config.ConfigPath,
		ProfilePath: s.config.ProfilePath,
		Strategy:    application.SuggestCurrent,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate suggestions: %w", err)
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal suggest result: %w", err)
	}

	return &mcp.ResourceContent{
		URI:      uri,
		MimeType: "application/json",
		Text:     string(data),
	}, nil
}

func (s *Server) handleConfigResource(ctx context.Context, uri string, params map[string]string) (*mcp.ResourceContent, error) {
	result, err := s.svc.Detect(ctx, application.DetectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to detect config: %w", err)
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	return &mcp.ResourceContent{
		URI:      uri,
		MimeType: "application/json",
		Text:     string(data),
	}, nil
}

func (s *Server) handleSuggest(ctx context.Context, input SuggestInput) (map[string]any, error) {
	defer traceTool("suggest")()
	if err := validateScopedInputs(
		namedPath{"configPath", input.ConfigPath},
		namedPath{"profile", input.Profile},
	); err != nil {
		return rejectionResponse(err), nil
	}

	strategy := application.SuggestCurrent
	switch input.Strategy {
	case "aggressive":
		strategy = application.SuggestAggressive
	case "conservative":
		strategy = application.SuggestConservative
	case "current", "":
		strategy = application.SuggestCurrent
	}

	configPath := s.resolveConfigPath(input.ConfigPath)

	opts := application.SuggestOptions{
		ConfigPath:  configPath,
		ProfilePath: coalesce(input.Profile, s.config.ProfilePath),
		Strategy:    strategy,
	}

	result, err := s.svc.Suggest(ctx, opts)

	if classified, ok := classifyRuntimeError(err); ok {
		return classified, nil
	}

	output := map[string]any{
		"passed":      err == nil,
		"suggestions": result.Suggestions,
	}

	if err != nil {
		output["passed"] = false
		output["error"] = err.Error()
		output["summary"] = "Failed to generate suggestions"
		return output, nil
	}

	// Write suggested config if requested
	if input.WriteConfig {
		// Apply suggested thresholds to the config
		suggestedConfig := applySuggestions(result.Config, result.Suggestions)

		// Backup existing config if it exists
		backupPath, backupErr := backupConfig(configPath)
		if backupErr != nil && !os.IsNotExist(backupErr) {
			output["error"] = fmt.Sprintf("failed to backup config: %v", backupErr)
			output["summary"] = "Failed to backup existing config"
			return output, nil
		}

		// Write new config
		if err := writeConfig(configPath, suggestedConfig); err != nil {
			output["error"] = err.Error()
			output["summary"] = "Failed to write config"
			return output, nil
		}

		output["configPath"] = configPath
		if backupPath != "" {
			output["backupPath"] = backupPath
		}
		output["summary"] = fmt.Sprintf("Applied %d threshold suggestions to %s", len(result.Suggestions), configPath)
	} else {
		output["summary"] = fmt.Sprintf("Generated %d threshold suggestions using %s strategy", len(result.Suggestions), strategy)
	}

	return output, nil
}

func (s *Server) handleDebt(ctx context.Context, input DebtInput) (map[string]any, error) {
	defer traceTool("debt")()
	if err := validateScopedInputs(
		namedPath{"configPath", input.ConfigPath},
		namedPath{"profile", input.Profile},
	); err != nil {
		return rejectionResponse(err), nil
	}

	opts := application.DebtOptions{
		ConfigPath:  s.resolveConfigPath(input.ConfigPath),
		ProfilePath: coalesce(input.Profile, s.config.ProfilePath),
		Output:      application.OutputJSON,
	}

	result, err := s.svc.Debt(ctx, opts)

	if classified, ok := classifyRuntimeError(err); ok {
		return classified, nil
	}

	v := resolveVerbosity(input.Verbosity)
	items, itemsCursor := applyDebtItemBudget(result.Items, v)
	output := map[string]any{
		"passed":      err == nil,
		"items":       sanitizeDebtItems(items),
		"totalDebt":   result.TotalDebt,
		"totalLines":  result.TotalLines,
		"healthScore": result.HealthScore,
	}
	if itemsCursor != "" {
		output["itemsNextCursor"] = itemsCursor
	}

	if err != nil {
		output["passed"] = false
		output["error"] = sanitizeOutputString(err.Error())
		output["summary"] = "Failed to calculate coverage debt"
	} else {
		if result.TotalDebt > 0 {
			output["summary"] = fmt.Sprintf("Coverage debt: %.1f%% gap across %d items (health score: %.0f/100)", result.TotalDebt, len(result.Items), result.HealthScore)
		} else {
			output["summary"] = fmt.Sprintf("No coverage debt - all thresholds met (health score: %.0f/100)", result.HealthScore)
		}
	}

	return output, nil
}

func (s *Server) handleCompare(ctx context.Context, input CompareInput) (map[string]any, error) {
	defer traceTool("compare")()
	if err := validateScopedInputs(
		namedPath{"configPath", input.ConfigPath},
		namedPath{"baseProfile", input.BaseProfile},
		namedPath{"headProfile", input.HeadProfile},
	); err != nil {
		return rejectionResponse(err), nil
	}

	if input.BaseProfile == "" {
		return map[string]any{
			"passed":  false,
			"error":   "baseProfile is required",
			"summary": "Missing required parameter",
		}, nil
	}

	opts := application.CompareOptions{
		ConfigPath:  s.resolveConfigPath(input.ConfigPath),
		BaseProfile: input.BaseProfile,
		HeadProfile: coalesce(input.HeadProfile, s.config.ProfilePath),
		Output:      application.OutputJSON,
	}

	result, err := s.svc.Compare(ctx, opts)

	if classified, ok := classifyRuntimeError(err); ok {
		return classified, nil
	}

	v := resolveVerbosity(input.Verbosity)
	improved, improvedCursor := applyFileDeltaBudget(result.Improved, v)
	regressed, regressedCursor := applyFileDeltaBudget(result.Regressed, v)
	output := map[string]any{
		"passed":       err == nil,
		"baseOverall":  result.BaseOverall,
		"headOverall":  result.HeadOverall,
		"delta":        result.Delta,
		"improved":     sanitizeFileDeltas(improved),
		"regressed":    sanitizeFileDeltas(regressed),
		"unchanged":    result.Unchanged,
		"domainDeltas": sanitizeDomainDeltas(result.DomainDeltas),
	}
	if improvedCursor != "" {
		output["improvedNextCursor"] = improvedCursor
	}
	if regressedCursor != "" {
		output["regressedNextCursor"] = regressedCursor
	}

	if err != nil {
		output["passed"] = false
		output["error"] = err.Error()
		output["summary"] = "Failed to compare coverage"
	} else {
		sign := "+"
		if result.Delta < 0 {
			sign = ""
		}
		output["summary"] = fmt.Sprintf("Coverage %s%.1f%% (%.1f%% → %.1f%%), %d improved, %d regressed",
			sign, result.Delta, result.BaseOverall, result.HeadOverall, len(result.Improved), len(result.Regressed))
	}

	return output, nil
}

func (s *Server) handlePRComment(ctx context.Context, input PRCommentInput) (map[string]any, error) {
	defer traceTool("pr-comment")()
	if err := validateScopedInputs(
		namedPath{"configPath", input.ConfigPath},
		namedPath{"profile", input.Profile},
		namedPath{"baseProfile", input.BaseProfile},
	); err != nil {
		return rejectionResponse(err), nil
	}

	// Parse provider
	var provider application.PRProvider
	switch strings.ToLower(input.Provider) {
	case "github":
		provider = application.ProviderGitHub
	case "gitlab":
		provider = application.ProviderGitLab
	case "bitbucket":
		provider = application.ProviderBitbucket
	case "auto", "":
		provider = application.ProviderAuto
	default:
		return map[string]any{
			"passed":  false,
			"error":   fmt.Sprintf("unknown provider %q (use github, gitlab, bitbucket, or auto)", input.Provider),
			"summary": "Invalid provider specified",
		}, nil
	}

	// Auto-detect owner/repo/PR from environment
	owner, repo, prNumber := detectPRContextMCP(provider, input.Owner, input.Repo, input.PRNumber)

	if prNumber == 0 {
		return map[string]any{
			"passed":  false,
			"error":   "prNumber is required (or set CI environment variables for auto-detection)",
			"summary": "Missing required parameter",
		}, nil
	}
	if owner == "" || repo == "" {
		return map[string]any{
			"passed":  false,
			"error":   "owner and repo are required (or set provider-specific environment variables)",
			"summary": "Missing required parameter",
		}, nil
	}

	if !input.DryRun {
		key := fmt.Sprintf("%s/%s/%s#%d", provider, owner, repo, prNumber)
		if err := s.prCommentLimit.allow(key, time.Now()); err != nil {
			return rejectionResponse(err), nil
		}
	}

	// Default UpdateExisting to true if not specified (nil means not provided)
	updateExisting := true
	if input.UpdateExisting != nil {
		updateExisting = *input.UpdateExisting
	}

	opts := application.PRCommentOptions{
		ConfigPath:     s.resolveConfigPath(input.ConfigPath),
		ProfilePath:    coalesce(input.Profile, s.config.ProfilePath),
		BaseProfile:    input.BaseProfile,
		Provider:       provider,
		PRNumber:       prNumber,
		Owner:          owner,
		Repo:           repo,
		UpdateExisting: updateExisting,
		DryRun:         input.DryRun,
	}

	result, err := s.svc.PRComment(ctx, opts)

	output := map[string]any{
		"passed":      err == nil,
		"commentBody": result.CommentBody,
		"provider":    string(provider),
	}

	if err != nil {
		output["passed"] = false
		output["error"] = err.Error()
		output["summary"] = "Failed to post PR comment"
	} else if input.DryRun {
		output["summary"] = "Generated PR comment (dry-run mode)"
	} else if result.Created {
		output["commentId"] = result.CommentID
		output["commentUrl"] = result.CommentURL
		output["summary"] = fmt.Sprintf("Created comment on PR #%d: %s", prNumber, result.CommentURL)
	} else {
		output["commentId"] = result.CommentID
		output["summary"] = fmt.Sprintf("Updated existing comment #%d on PR #%d", result.CommentID, prNumber)
	}

	return output, nil
}

// detectPRContextMCP auto-detects owner, repo, and PR number from environment variables.
func detectPRContextMCP(provider application.PRProvider, owner, repo string, prNumber int) (string, string, int) {
	// If all values are already provided, return them
	if owner != "" && repo != "" && prNumber != 0 {
		return owner, repo, prNumber
	}

	// GitHub: GITHUB_REPOSITORY=owner/repo
	if (provider == application.ProviderGitHub || provider == application.ProviderAuto) && (owner == "" || repo == "") {
		if ghRepo := os.Getenv("GITHUB_REPOSITORY"); ghRepo != "" {
			parts := strings.SplitN(ghRepo, "/", 2)
			if len(parts) == 2 {
				if owner == "" {
					owner = parts[0]
				}
				if repo == "" {
					repo = parts[1]
				}
			}
		}
	}

	// GitLab: CI_PROJECT_NAMESPACE and CI_PROJECT_NAME
	if (provider == application.ProviderGitLab || provider == application.ProviderAuto) && (owner == "" || repo == "") {
		if ns := os.Getenv("CI_PROJECT_NAMESPACE"); ns != "" && owner == "" {
			owner = ns
		}
		if name := os.Getenv("CI_PROJECT_NAME"); name != "" && repo == "" {
			repo = name
		}
		// GitLab can also auto-detect MR number
		if prNumber == 0 {
			if mrIID := os.Getenv("CI_MERGE_REQUEST_IID"); mrIID != "" {
				_, _ = fmt.Sscanf(mrIID, "%d", &prNumber) // #nosec G104 - parse failure leaves prNumber as 0, which is acceptable
			}
		}
	}

	// Bitbucket: BITBUCKET_WORKSPACE and BITBUCKET_REPO_SLUG
	if (provider == application.ProviderBitbucket || provider == application.ProviderAuto) && (owner == "" || repo == "") {
		if ws := os.Getenv("BITBUCKET_WORKSPACE"); ws != "" && owner == "" {
			owner = ws
		}
		if slug := os.Getenv("BITBUCKET_REPO_SLUG"); slug != "" && repo == "" {
			repo = slug
		}
		// Bitbucket can also auto-detect PR number
		if prNumber == 0 {
			if prID := os.Getenv("BITBUCKET_PR_ID"); prID != "" {
				_, _ = fmt.Sscanf(prID, "%d", &prNumber) // #nosec G104 - parse failure leaves prNumber as 0, which is acceptable
			}
		}
	}

	return owner, repo, prNumber
}

// Helper functions for config management

// resolveConfigPath returns the config path to use.
// If inputPath is specified and exists, use it.
// If inputPath is specified but doesn't exist, try auto-detection.
// If inputPath is empty, use server default.
func (s *Server) resolveConfigPath(inputPath string) string {
	// Use input path if provided
	if inputPath != "" {
		// If input path exists, use it directly
		if _, err := os.Stat(inputPath); err == nil {
			return inputPath
		}
		// Input path doesn't exist, try auto-detection
		if foundPath, findErr := config.FindConfigFrom(""); findErr == nil {
			return foundPath
		}
		// Auto-detection failed, return input path (will produce clear error)
		return inputPath
	}

	// No input path, use server default
	return s.config.ConfigPath
}

// applySuggestions applies the suggested thresholds to the config.
func applySuggestions(cfg application.Config, suggestions []application.Suggestion) application.Config {
	// Create a map for quick lookup
	suggestedMins := make(map[string]float64)
	for _, s := range suggestions {
		suggestedMins[s.Domain] = s.SuggestedMin
	}

	// Apply suggestions to domains
	for i := range cfg.Policy.Domains {
		if min, ok := suggestedMins[cfg.Policy.Domains[i].Name]; ok {
			minVal := min
			cfg.Policy.Domains[i].Min = &minVal
		}
	}

	return cfg
}

// backupConfig creates a backup of the existing config file.
// Returns the backup path and any error.
func backupConfig(configPath string) (string, error) {
	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return "", err
	}

	// Read original content
	content, err := os.ReadFile(configPath) // #nosec G304 - path from trusted config
	if err != nil {
		return "", fmt.Errorf("read config: %w", err)
	}

	// Create backup with timestamp
	backupPath := configPath + ".backup"
	if err := os.WriteFile(backupPath, content, 0o600); err != nil {
		return "", fmt.Errorf("write backup: %w", err)
	}

	return backupPath, nil
}

// writeConfig writes the config to the specified path.
func writeConfig(configPath string, cfg application.Config) error {
	// Validate path
	cleanPath, err := pathutil.ValidatePath(configPath)
	if err != nil {
		return fmt.Errorf("invalid config path: %w", err)
	}

	file, err := os.Create(cleanPath) // #nosec G304 - path is validated above
	if err != nil {
		return fmt.Errorf("create config: %w", err)
	}
	defer file.Close()

	if err := config.Write(file, cfg); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

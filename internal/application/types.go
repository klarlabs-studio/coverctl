package application

import (
	"context"
	"errors"
	"io"

	"go.klarlabs.de/coverctl/internal/domain"
)

type OutputFormat string

const (
	OutputText  OutputFormat = "text"
	OutputJSON  OutputFormat = "json"
	OutputHTML  OutputFormat = "html"
	OutputBrief OutputFormat = "brief"
)

// Language represents a programming language.
type Language string

const (
	// LanguageAuto auto-detects the project language.
	LanguageAuto Language = "auto"
	// LanguageGo is the Go programming language.
	LanguageGo Language = "go"
	// LanguagePython is the Python programming language.
	LanguagePython Language = "python"
	// LanguageTypeScript is the TypeScript programming language.
	LanguageTypeScript Language = "typescript"
	// LanguageJavaScript is the JavaScript programming language.
	LanguageJavaScript Language = "javascript"
	// LanguageJava is the Java programming language.
	LanguageJava Language = "java"
	// LanguageRust is the Rust programming language.
	LanguageRust Language = "rust"
	// LanguageCSharp is the C# programming language.
	LanguageCSharp Language = "csharp"
	// LanguageCpp is the C/C++ programming language.
	LanguageCpp Language = "cpp"
	// LanguagePHP is the PHP programming language.
	LanguagePHP Language = "php"
	// LanguageRuby is the Ruby programming language.
	LanguageRuby Language = "ruby"
	// LanguageSwift is the Swift programming language.
	LanguageSwift Language = "swift"
	// LanguageDart is the Dart programming language.
	LanguageDart Language = "dart"
	// LanguageScala is the Scala programming language.
	LanguageScala Language = "scala"
	// LanguageElixir is the Elixir programming language.
	LanguageElixir Language = "elixir"
	// LanguageShell is the Shell/Bash scripting language.
	LanguageShell Language = "shell"
)

// LanguageMarker pairs a project-root filename with a priority used during
// language detection. Higher priority wins when multiple markers are present
// (e.g. tsconfig.json + package.json → TypeScript wins over JavaScript).
type LanguageMarker struct {
	Filename string
	Priority int
}

// LanguageDef is the single source of truth about a supported language.
//
// Adding a new language used to require coordinated edits across eight
// files (engineering review R4). Now: append one entry here, register the
// runner in the runners package, ship the schema-enum update generated from
// this list. Three places, one canonical source.
//
// Downstream consumers (parsers/detector for markers + format + profile
// paths, application/service for source extensions, mcp + cli for surfaced
// language lists) all derive their lookup tables from this slice.
type LanguageDef struct {
	Code             Language
	SourceExtensions []string
	Markers          []LanguageMarker
	DefaultFormat    Format
	ProfilePaths     []string
}

// Languages is the canonical registry. ORDER MATTERS where ambiguity is
// possible: detection picks the highest-priority marker, but if priorities
// tie, the earlier entry in this slice wins. Keep more-specific languages
// (TypeScript) ahead of more-generic ones (JavaScript) when their marker
// sets overlap.
var Languages = []LanguageDef{
	{
		Code:             LanguageGo,
		SourceExtensions: []string{".go"},
		Markers: []LanguageMarker{
			{Filename: "go.mod", Priority: 100},
			{Filename: "go.sum", Priority: 90},
		},
		DefaultFormat: FormatGo,
		ProfilePaths:  []string{"coverage.out", "cover.out", "c.out"},
	},
	{
		Code:             LanguagePython,
		SourceExtensions: []string{".py"},
		Markers: []LanguageMarker{
			{Filename: "pyproject.toml", Priority: 100},
			{Filename: "setup.py", Priority: 90},
			{Filename: "Pipfile", Priority: 85},
			{Filename: "poetry.lock", Priority: 85},
			{Filename: "requirements.txt", Priority: 80},
		},
		DefaultFormat: FormatCobertura,
		ProfilePaths:  []string{"coverage.xml", ".coverage", "coverage.info", "htmlcov/", "coverage-report"},
	},
	{
		Code:             LanguageTypeScript,
		SourceExtensions: []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"},
		Markers: []LanguageMarker{
			{Filename: "tsconfig.json", Priority: 100},
		},
		DefaultFormat: FormatLCOV,
		ProfilePaths:  []string{"coverage/lcov.info", "coverage/coverage.json", "coverage/cobertura.xml", ".nyc_output/"},
	},
	{
		Code:             LanguageJavaScript,
		SourceExtensions: []string{".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx"},
		Markers: []LanguageMarker{
			{Filename: "package.json", Priority: 90},
			{Filename: "yarn.lock", Priority: 80},
			{Filename: "pnpm-lock.yaml", Priority: 80},
			{Filename: "package-lock.json", Priority: 80},
		},
		DefaultFormat: FormatLCOV,
		ProfilePaths:  []string{"coverage/lcov.info", "coverage/coverage.json", "coverage/cobertura.xml", ".nyc_output/"},
	},
	{
		Code:             LanguageJava,
		SourceExtensions: []string{".java", ".kt"},
		Markers: []LanguageMarker{
			{Filename: "pom.xml", Priority: 100},
			{Filename: "build.gradle", Priority: 100},
			{Filename: "build.gradle.kts", Priority: 100},
			{Filename: "settings.gradle", Priority: 90},
			{Filename: "settings.gradle.kts", Priority: 90},
		},
		DefaultFormat: FormatJaCoCo,
		ProfilePaths: []string{
			"target/site/jacoco/jacoco.xml",
			"build/reports/jacoco/test/jacocoTestReport.xml",
			"target/site/cobertura/coverage.xml",
			"build/reports/cobertura/coverage.xml",
		},
	},
	{
		Code:             LanguageRust,
		SourceExtensions: []string{".rs"},
		Markers: []LanguageMarker{
			{Filename: "Cargo.toml", Priority: 100},
			{Filename: "Cargo.lock", Priority: 90},
		},
		DefaultFormat: FormatLCOV,
		ProfilePaths:  []string{"target/coverage/lcov.info", "target/coverage/cobertura.xml", "coverage/lcov.info"},
	},
	{
		Code:             LanguageCSharp,
		SourceExtensions: []string{".cs"},
		Markers: []LanguageMarker{
			{Filename: "Directory.Build.props", Priority: 100},
			{Filename: "global.json", Priority: 90},
		},
		DefaultFormat: FormatCobertura,
		ProfilePaths:  []string{"TestResults/coverage.cobertura.xml"},
	},
	{
		Code:             LanguageCpp,
		SourceExtensions: []string{".c", ".cpp", ".cc", ".cxx", ".h", ".hpp"},
		Markers: []LanguageMarker{
			{Filename: "CMakeLists.txt", Priority: 100},
			{Filename: "meson.build", Priority: 95},
			{Filename: "configure.ac", Priority: 90},
		},
		DefaultFormat: FormatLCOV,
		ProfilePaths:  []string{"coverage/lcov.info", "build/coverage/lcov.info"},
	},
	{
		Code:             LanguagePHP,
		SourceExtensions: []string{".php"},
		Markers: []LanguageMarker{
			{Filename: "composer.json", Priority: 100},
			{Filename: "phpunit.xml", Priority: 95},
			{Filename: "composer.lock", Priority: 90},
			{Filename: "phpunit.xml.dist", Priority: 90},
		},
		DefaultFormat: FormatCobertura,
		ProfilePaths:  []string{"coverage.xml"},
	},
	{
		Code:             LanguageRuby,
		SourceExtensions: []string{".rb"},
		Markers: []LanguageMarker{
			{Filename: "Gemfile", Priority: 100},
			{Filename: "Gemfile.lock", Priority: 90},
			{Filename: "Rakefile", Priority: 85},
		},
		DefaultFormat: FormatLCOV,
		ProfilePaths:  []string{"coverage/lcov.info"},
	},
	{
		Code:             LanguageSwift,
		SourceExtensions: []string{".swift"},
		Markers: []LanguageMarker{
			{Filename: "Package.swift", Priority: 100},
		},
		DefaultFormat: FormatLCOV,
		ProfilePaths:  []string{"coverage/lcov.info"},
	},
	{
		Code:             LanguageDart,
		SourceExtensions: []string{".dart"},
		Markers: []LanguageMarker{
			{Filename: "pubspec.yaml", Priority: 100},
			{Filename: "pubspec.lock", Priority: 90},
		},
		DefaultFormat: FormatLCOV,
		ProfilePaths:  []string{"coverage/lcov.info"},
	},
	{
		Code:             LanguageScala,
		SourceExtensions: []string{".scala", ".sc"},
		Markers: []LanguageMarker{
			{Filename: "build.sbt", Priority: 100},
		},
		DefaultFormat: FormatCobertura,
		ProfilePaths: []string{
			"target/scala-2.13/scoverage-report/scoverage.xml",
			"target/scala-3/scoverage-report/scoverage.xml",
		},
	},
	{
		Code:             LanguageElixir,
		SourceExtensions: []string{".ex", ".exs"},
		Markers: []LanguageMarker{
			{Filename: "mix.exs", Priority: 100},
			{Filename: "mix.lock", Priority: 90},
		},
		DefaultFormat: FormatLCOV,
		ProfilePaths:  []string{"cover/lcov.info"},
	},
	{
		Code:             LanguageShell,
		SourceExtensions: []string{".sh", ".bash"},
		Markers:          nil, // no canonical project marker; detected via file extension only
		DefaultFormat:    FormatCobertura,
		ProfilePaths:     []string{"coverage/cobertura.xml"},
	},
}

// LookupLanguage returns the LanguageDef for the given code, or false if no
// such language is registered. O(n) over Languages — fine for n=15.
func LookupLanguage(code Language) (LanguageDef, bool) {
	for _, def := range Languages {
		if def.Code == code {
			return def, true
		}
	}
	return LanguageDef{}, false
}

// Format represents a coverage profile format.
type Format string

const (
	// FormatAuto auto-detects the coverage format.
	FormatAuto Format = "auto"
	// FormatGo is the Go coverage profile format.
	FormatGo Format = "go"
	// FormatLCOV is the LCOV coverage format.
	FormatLCOV Format = "lcov"
	// FormatCobertura is the Cobertura XML coverage format.
	FormatCobertura Format = "cobertura"
	// FormatJaCoCo is the JaCoCo XML coverage format.
	FormatJaCoCo Format = "jacoco"
	// FormatLLVMCov is the LLVM coverage JSON format.
	FormatLLVMCov Format = "llvm-cov"
)

var ErrConfigNotFound = errors.New("config not found")

// Config represents validated, application-ready configuration.
type Config struct {
	Version     int
	Language    Language      // Project language (auto-detected if empty)
	Profile     ProfileConfig // Coverage profile configuration
	Policy      domain.Policy
	Exclude     []string
	Files       []domain.FileRule
	Diff        DiffConfig
	Merge       MergeConfig
	Integration IntegrationConfig
	Annotations AnnotationsConfig
}

// ProfileConfig configures coverage profile handling.
type ProfileConfig struct {
	Format Format // Coverage format (auto, go, lcov, cobertura, jacoco)
	Path   string // Default profile path
}

type FileRule = domain.FileRule

type DiffConfig struct {
	Enabled bool
	Base    string
}

type MergeConfig struct {
	Profiles []string
}

type IntegrationConfig struct {
	Enabled  bool
	Packages []string
	RunArgs  []string
	CoverDir string
	Profile  string
}

type AnnotationsConfig struct {
	Enabled bool
}

type ConfigLoader interface {
	Load(path string) (Config, error)
	Exists(path string) (bool, error)
}

type Autodetector interface {
	Detect() (Config, error)
}

type DomainResolver interface {
	Resolve(ctx context.Context, domains []domain.Domain) (map[string][]string, error)
	ModuleRoot(ctx context.Context) (string, error)
	ModulePath(ctx context.Context) (string, error)
}

// CoverageRunner executes tests with coverage instrumentation for a specific language.
// Implementations exist for Go, Python, Node.js, Rust, and Java.
type CoverageRunner interface {
	// Run executes tests with coverage and returns the profile path.
	Run(ctx context.Context, opts RunOptions) (string, error)
	// RunIntegration runs integration tests with coverage collection.
	RunIntegration(ctx context.Context, opts IntegrationOptions) (string, error)
	// Name returns the runner's identifier (e.g., "go", "python", "nodejs").
	Name() string
	// Language returns the language this runner supports.
	Language() Language
	// Detect checks if this runner can handle the current project.
	// Returns true if the project uses this runner's language.
	Detect(projectDir string) bool
}

// RunnerRegistry manages multiple coverage runners and selects the appropriate one.
type RunnerRegistry interface {
	// GetRunner returns a runner for the specified language.
	GetRunner(lang Language) (CoverageRunner, error)
	// DetectRunner auto-detects the appropriate runner for the project directory.
	DetectRunner(projectDir string) (CoverageRunner, error)
	// SupportedLanguages returns all languages with available runners.
	SupportedLanguages() []Language
}

// ProfileParser parses coverage profiles into domain stats.
// Implementations exist for each supported format.
type ProfileParser interface {
	// Parse reads a coverage profile and returns file-level stats.
	Parse(path string) (map[string]domain.CoverageStat, error)
	// ParseAll merges multiple profiles into unified stats.
	ParseAll(paths []string) (map[string]domain.CoverageStat, error)
	// Format returns the format this parser handles.
	Format() Format
}

type DiffProvider interface {
	ChangedFiles(ctx context.Context, base string) ([]string, error)
}

type AnnotationScanner interface {
	Scan(ctx context.Context, moduleRoot string, files []string) (map[string]Annotation, error)
}

type Reporter interface {
	Write(w io.Writer, result domain.Result, format OutputFormat) error
}

type RunOptions struct {
	Domains     []domain.Domain
	ProfilePath string
	BuildFlags  BuildFlags // Build and test flags
	Packages    []string   // Specific packages to test (empty = all packages via ./...)
}

// BuildFlags contains options passed to go test
type BuildFlags struct {
	Tags     string   // Build tags (e.g., "integration,e2e")
	Race     bool     // Enable race detector
	Short    bool     // Skip long-running tests
	Verbose  bool     // Verbose test output
	Run      string   // Run only tests matching pattern
	Timeout  string   // Test timeout (e.g., "10m", "1h")
	TestArgs []string // Additional arguments passed to go test
}

type IntegrationOptions struct {
	Domains    []domain.Domain
	Packages   []string
	RunArgs    []string
	CoverDir   string
	Profile    string
	BuildFlags BuildFlags // Build and test flags
}

type Annotation struct {
	Ignore bool
	Domain string
}

type IgnoreOptions struct {
	ConfigPath string
}

type BadgeOptions struct {
	ConfigPath  string
	ProfilePath string
	Output      string
	Label       string
	Style       string
}

type TrendOptions struct {
	ConfigPath  string
	ProfilePath string
	HistoryPath string
	Output      OutputFormat
	Days        int // Number of days to analyze (0 = all)
}

type RecordOptions struct {
	ConfigPath  string
	ProfilePath string
	HistoryPath string
	Commit      string
	Branch      string
	Run         bool
	Domains     []string
	BuildFlags  BuildFlags
	Language    Language
}

type RecordResult struct {
	Warnings []string
}

type HistoryStore interface {
	Load() (domain.History, error)
	Save(h domain.History) error
	Append(entry domain.HistoryEntry) error
}

type SuggestOptions struct {
	ConfigPath  string
	ProfilePath string
	Strategy    SuggestStrategy
}

type SuggestStrategy string

const (
	// SuggestCurrent suggests thresholds slightly below current coverage
	SuggestCurrent SuggestStrategy = "current"
	// SuggestAggressive suggests higher thresholds to push for improvement
	SuggestAggressive SuggestStrategy = "aggressive"
	// SuggestConservative suggests lower thresholds for gradual improvement
	SuggestConservative SuggestStrategy = "conservative"
)

type Suggestion struct {
	Domain         string
	CurrentPercent float64
	CurrentMin     float64
	SuggestedMin   float64
	Reason         string
}

// FileWatcher provides file change notifications.
type FileWatcher interface {
	WatchDir(root string) error
	Events(ctx context.Context) <-chan struct{}
	Close() error
}

// WatchOptions configures watch mode behavior.
type WatchOptions struct {
	ConfigPath string
	Profile    string
	Domains    []string
	Clear      bool       // Clear terminal before each run
	BuildFlags BuildFlags // Build and test flags
}

// DebtOptions configures the coverage debt report.
type DebtOptions struct {
	ConfigPath  string
	ProfilePath string
	Output      OutputFormat
}

// DebtItem represents a single coverage debt item.
type DebtItem struct {
	Name      string  // Domain or file name
	Type      string  // "domain" or "file"
	Current   float64 // Current coverage percentage
	Required  float64 // Required minimum coverage
	Shortfall float64 // How much coverage is missing (required - current)
	Lines     int     // Estimated lines of code needing tests
}

// DebtResult contains the overall coverage debt analysis.
type DebtResult struct {
	Items       []DebtItem
	TotalDebt   float64 // Sum of all shortfalls
	TotalLines  int     // Total estimated lines needing tests
	HealthScore float64 // 0-100 score (higher is better)
}

// CompareOptions configures the coverage comparison.
type CompareOptions struct {
	ConfigPath  string
	BaseProfile string // Path to base coverage profile
	HeadProfile string // Path to head coverage profile (or "current" to run tests)
	Output      OutputFormat
}

// CompareResult contains the comparison between two coverage profiles.
type CompareResult struct {
	BaseOverall  float64            `json:"baseOverall"`
	HeadOverall  float64            `json:"headOverall"`
	Delta        float64            `json:"delta"`
	Improved     []FileDelta        `json:"improved"`
	Regressed    []FileDelta        `json:"regressed"`
	Unchanged    int                `json:"unchanged"`
	DomainDeltas map[string]float64 `json:"domainDeltas"`
}

// FileDelta represents a coverage change for a single file.
type FileDelta struct {
	File    string  `json:"file"`
	BasePct float64 `json:"basePct"`
	HeadPct float64 `json:"headPct"`
	Delta   float64 `json:"delta"`
}

// PRProvider represents a git hosting provider.
type PRProvider string

const (
	// ProviderGitHub is GitHub.com or GitHub Enterprise
	ProviderGitHub PRProvider = "github"
	// ProviderGitLab is GitLab.com or self-hosted GitLab
	ProviderGitLab PRProvider = "gitlab"
	// ProviderBitbucket is Bitbucket Cloud
	ProviderBitbucket PRProvider = "bitbucket"
	// ProviderAuto auto-detects the provider from environment
	ProviderAuto PRProvider = "auto"
)

// PRCommentOptions configures the PR comment feature.
type PRCommentOptions struct {
	ConfigPath     string
	ProfilePath    string
	BaseProfile    string     // Base profile for comparison (optional)
	Provider       PRProvider // Git hosting provider (auto-detected if empty)
	PRNumber       int        // PR/MR number to comment on
	Owner          string     // Repository owner/namespace
	Repo           string     // Repository name
	ProjectID      string     // GitLab project ID (alternative to owner/repo)
	UpdateExisting bool       // Update existing comment instead of creating new
	DryRun         bool       // Just generate comment, don't post
}

// PRCommentResult contains the result of a PR comment operation.
type PRCommentResult struct {
	CommentID   int64  `json:"commentId,omitempty"`
	CommentURL  string `json:"commentUrl,omitempty"`
	CommentBody string `json:"commentBody"`
	Created     bool   `json:"created"` // true if created, false if updated
}

// PRClient provides PR comment operations for any git hosting provider.
type PRClient interface {
	// Provider returns the provider type
	Provider() PRProvider
	// FindCoverageComment finds an existing coverage comment on a PR/MR
	FindCoverageComment(ctx context.Context, owner, repo string, prNumber int) (int64, error)
	// CreateComment creates a new comment on a PR/MR
	CreateComment(ctx context.Context, owner, repo string, prNumber int, body string) (int64, string, error)
	// UpdateComment updates an existing comment
	UpdateComment(ctx context.Context, owner, repo string, commentID int64, body string) error
}

// GitHubClient provides GitHub API operations (alias for backward compatibility).
type GitHubClient = PRClient

// CommentFormatter generates PR comment content.
type CommentFormatter interface {
	// FormatCoverageComment generates markdown for a coverage PR comment
	FormatCoverageComment(result domain.Result, comparison *CompareResult) string
}

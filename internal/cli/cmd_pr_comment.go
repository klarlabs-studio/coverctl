package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"go.klarlabs.de/coverctl/internal/application"
)

// runPRComment implements `coverctl pr-comment`.
func runPRComment(ctx context.Context, args []string, stdout, stderr io.Writer, svc Service, global GlobalOptions) int {
	fs := flag.NewFlagSet("pr-comment", flag.ContinueOnError)
	fs.Usage = func() { commandHelp("pr-comment", stderr) }
	configPath := fs.String("config", ".coverctl.yaml", "Config file path")
	fs.StringVar(configPath, "c", ".coverctl.yaml", "Config file path (shorthand)")
	profilePath := fs.String("profile", ".cover/coverage.out", "Coverage profile path")
	fs.StringVar(profilePath, "p", ".cover/coverage.out", "Coverage profile path (shorthand)")
	baseProfile := fs.String("base", "", "Base coverage profile for comparison (optional)")
	prNumber := fs.Int("pr", 0, "Pull request/MR number (required)")
	owner := fs.String("owner", "", "Repository owner/namespace (auto-detected from env)")
	repo := fs.String("repo", "", "Repository name (auto-detected from env)")
	provider := fs.String("provider", "auto", "Git provider: github, gitlab, bitbucket, or auto")
	updateExisting := fs.Bool("update", true, "Update existing comment instead of creating new")
	dryRun := fs.Bool("dry-run", false, "Generate comment without posting")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	var prProvider application.PRProvider
	switch strings.ToLower(*provider) {
	case "github":
		prProvider = application.ProviderGitHub
	case "gitlab":
		prProvider = application.ProviderGitLab
	case "bitbucket":
		prProvider = application.ProviderBitbucket
	case "auto", "":
		prProvider = application.ProviderAuto
	default:
		fmt.Fprintf(stderr, "Error: unknown provider %q (use github, gitlab, bitbucket, or auto)\n", *provider)
		return 2
	}

	ownerVal, repoVal, prNum := detectPRContext(prProvider, *owner, *repo, *prNumber)
	if prNum == 0 {
		fmt.Fprintln(stderr, "Error: --pr flag is required")
		fs.Usage()
		return 2
	}
	if ownerVal == "" || repoVal == "" {
		fmt.Fprintln(stderr, "Error: --owner and --repo flags are required (or set provider-specific env vars)")
		fmt.Fprintln(stderr, "  GitHub: GITHUB_REPOSITORY=owner/repo")
		fmt.Fprintln(stderr, "  GitLab: CI_PROJECT_NAMESPACE and CI_PROJECT_NAME")
		fmt.Fprintln(stderr, "  Bitbucket: BITBUCKET_WORKSPACE and BITBUCKET_REPO_SLUG")
		return 2
	}

	result, err := svc.PRComment(ctx, application.PRCommentOptions{
		ConfigPath:     *configPath,
		ProfilePath:    *profilePath,
		BaseProfile:    *baseProfile,
		Provider:       prProvider,
		PRNumber:       prNum,
		Owner:          ownerVal,
		Repo:           repoVal,
		UpdateExisting: *updateExisting,
		DryRun:         *dryRun,
	})
	if err != nil {
		return exitCodeWithCI(err, 3, stderr, global)
	}

	if *dryRun {
		fmt.Fprintln(stdout, result.CommentBody)
	} else if result.Created {
		fmt.Fprintf(stdout, "Created comment: %s\n", result.CommentURL)
	} else {
		fmt.Fprintf(stdout, "Updated comment #%d\n", result.CommentID)
	}
	return 0
}

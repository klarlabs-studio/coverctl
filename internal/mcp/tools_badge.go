package mcp

import (
	"context"
	"fmt"

	"go.klarlabs.de/coverctl/internal/application"
)

// handleBadge handles the `badge` tool: generate an SVG coverage badge.
func (s *Server) handleBadge(ctx context.Context, input BadgeInput) (map[string]any, error) {
	defer traceTool("badge")()
	if err := validateScopedInputs(
		namedPath{"configPath", input.ConfigPath},
		namedPath{"profile", input.Profile},
		namedPath{"output", input.Output},
	); err != nil {
		return rejectionResponse(err), nil
	}

	opts := application.BadgeOptions{
		ConfigPath:  s.resolveConfigPath(input.ConfigPath),
		ProfilePath: coalesce(input.Profile, s.config.ProfilePath),
		Output:      coalesce(input.Output, "svg"),
		Label:       coalesce(input.Label, "coverage"),
		Style:       coalesce(input.Style, "flat"),
	}

	result, err := s.svc.Badge(ctx, opts)
	output := map[string]any{
		"passed":  err == nil,
		"percent": result.Percent,
		"summary": fmt.Sprintf("Coverage: %.1f%%", result.Percent),
	}
	if err != nil {
		output["passed"] = false
		output["error"] = err.Error()
		output["summary"] = "Failed to generate badge"
	} else if input.Output != "" {
		output["outputPath"] = input.Output
	}
	return output, nil
}

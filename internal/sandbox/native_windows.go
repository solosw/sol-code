//go:build windows

package sandbox

import (
	"context"
	"fmt"
	"strings"
)

// Windows commands currently use the workspace-scoped executor. This preserves
// the native sandbox API while the platform-specific confinement mechanism is
// supplied by Windows process controls.
func (s *Sandbox) confineAndRun(ctx context.Context, cmd Command) (Result, error) {
	result, err := s.unconfinedRun(ctx, cmd)
	warning := "[warning: windows sandbox not yet implemented, running unconfined]"
	if result.Stderr == "" {
		result.Stderr = warning
	} else {
		result.Stderr = strings.TrimRight(result.Stderr, "\n") + "\n" + warning
	}
	if err != nil {
		return result, fmt.Errorf("native sandbox (windows): %w", err)
	}
	return result, nil
}

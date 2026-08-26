package tool

import (
	"context"
	"os"
	"path/filepath"
)

// ReadTextFileContent uses an ACP client filesystem when this invocation has an
// advertised read capability. Other callers retain the local filesystem path.
func ReadTextFileContent(ctx context.Context, uctx *UseContext, path string) ([]byte, error) {
	if uctx != nil && uctx.TextFileSystem != nil && uctx.TextFileSystem.CanReadTextFile() {
		content, err := uctx.TextFileSystem.ReadTextFile(ctx, path)
		if err != nil {
			return nil, err
		}
		return []byte(content), nil
	}
	return os.ReadFile(path)
}

// WriteTextFileContent uses an ACP client filesystem when this invocation has an
// advertised write capability. The local fallback creates parent directories,
// matching the existing Write tool behavior.
func WriteTextFileContent(ctx context.Context, uctx *UseContext, path, content string) error {
	if uctx != nil && uctx.TextFileSystem != nil && uctx.TextFileSystem.CanWriteTextFile() {
		return uctx.TextFileSystem.WriteTextFile(ctx, path, content)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

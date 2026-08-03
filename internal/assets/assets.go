// Package assets provides embedded filesystem access to all injectable content:
// skills, orchestrator prompts, conventions, commands, and protocols.
package assets

import (
	"embed"
	"fmt"
	"io/fs"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

//go:embed all:skills all:generic all:opencode
var FS embed.FS

// commandAssetSpecs enumerates every retained command as a typed AssetSpec so
// that the typed installation path can compile and install them through the
// same plan/apply/receipt flow as all other retained assets.
var commandAssetSpecs = []ir.AssetSpec{
	{ID: "command/bootstrap", Class: ir.AssetCommand, SourcePath: "opencode/commands/bootstrap.md", Required: true, MaxTokens: 300},
	{ID: "command/investigate", Class: ir.AssetCommand, SourcePath: "opencode/commands/investigate.md", Required: true, MaxTokens: 300},
	{ID: "command/new-change", Class: ir.AssetCommand, SourcePath: "opencode/commands/new-change.md", Required: true, MaxTokens: 300},
	{ID: "command/continue", Class: ir.AssetCommand, SourcePath: "opencode/commands/continue.md", Required: true, MaxTokens: 300},
	{ID: "command/fast-forward", Class: ir.AssetCommand, SourcePath: "opencode/commands/fast-forward.md", Required: true, MaxTokens: 300},
	{ID: "command/implement", Class: ir.AssetCommand, SourcePath: "opencode/commands/implement.md", Required: true, MaxTokens: 300},
	{ID: "command/validate", Class: ir.AssetCommand, SourcePath: "opencode/commands/validate.md", Required: true, MaxTokens: 300},
	{ID: "command/finalize", Class: ir.AssetCommand, SourcePath: "opencode/commands/finalize.md", Required: true, MaxTokens: 300},
	{ID: "command/debate", Class: ir.AssetCommand, SourcePath: "opencode/commands/debate.md", Required: false, MaxTokens: 300},
	{ID: "command/monitor", Class: ir.AssetCommand, SourcePath: "opencode/commands/monitor.md", Required: false, MaxTokens: 300},
}

// CommandAssetSpecs returns the typed AssetSpec entries for every retained
// command. Callers fingerprint each spec's content before installation.
func CommandAssetSpecs() []ir.AssetSpec {
	out := make([]ir.AssetSpec, len(commandAssetSpecs))
	copy(out, commandAssetSpecs)
	return out
}

// CommandCatalog returns a validated AssetCatalog containing all retained
// commands plus the mandatory root-index placeholder so the catalog satisfies
// ir.AssetCatalog.Validate without a separate compilation step.
func CommandCatalog(rootIndex ir.AssetSpec) (ir.AssetCatalog, error) {
	if err := rootIndex.Validate(); err != nil {
		return ir.AssetCatalog{}, fmt.Errorf("command catalog root index: %w", err)
	}
	assets := make([]ir.AssetSpec, 0, len(commandAssetSpecs)+1)
	assets = append(assets, rootIndex)
	assets = append(assets, commandAssetSpecs...)
	cat := ir.AssetCatalog{
		SchemaVersion: ir.MustParseVersion("1.0.0"),
		Assets:        assets,
	}
	if err := cat.Validate(); err != nil {
		return ir.AssetCatalog{}, err
	}
	return cat, nil
}

// Read returns the content of an embedded asset file.
func Read(path string) (string, error) {
	data, err := fs.ReadFile(FS, path)
	if err != nil {
		return "", fmt.Errorf("read embedded asset %q: %w", path, err)
	}
	return string(data), nil
}

// ReadBytes returns the raw bytes of an embedded asset file.
func ReadBytes(path string) ([]byte, error) {
	data, err := fs.ReadFile(FS, path)
	if err != nil {
		return nil, fmt.Errorf("read embedded asset %q: %w", path, err)
	}
	return data, nil
}

// ListDir returns all entries in an embedded directory.
func ListDir(path string) ([]fs.DirEntry, error) {
	return fs.ReadDir(FS, path)
}

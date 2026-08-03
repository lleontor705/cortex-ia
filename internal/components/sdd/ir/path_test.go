package ir

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssetPathResolveRejectsUnsafePortableDestinations(t *testing.T) {
	root := t.TempDir()
	for _, relative := range []string{"../escape", "/absolute", `C:\absolute`, `\\server\share`, "a/../../escape"} {
		t.Run(relative, func(t *testing.T) {
			if _, err := (AssetPath{Scope: ScopeWorkflowRoot, Relative: relative}).Resolve(root); err == nil {
				t.Fatalf("Resolve(%q) unexpectedly succeeded", relative)
			}
		})
	}
	if _, err := (AssetPath{Scope: ScopeExternalConfig, Absolute: filepath.Join(root, "external"), Relative: "settings.json"}).Resolve(root); err == nil {
		t.Fatal("portable Resolve accepted external-config path")
	}
}

func TestAssetPathResolveProtectsSymlinkAndCaseFoldContainment(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err == nil {
		if _, err := (AssetPath{Scope: ScopeWorkflowRoot, Relative: "link/file"}).Resolve(root); err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("symlink escape error = %v", err)
		}
	}
	got, err := (AssetPath{Scope: ScopeWorkflowRoot, Relative: "nested/file.md"}).Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	if !contained(root, got) {
		t.Fatalf("resolved path %q is outside root %q", got, root)
	}
}

func TestAssetPathResolveExternalRequiresAbsoluteRoot(t *testing.T) {
	if _, err := (AssetPath{Scope: ScopeExternalConfig, Absolute: "relative", Relative: "settings.json"}).ResolveExternal(); err == nil {
		t.Fatal("relative external root unexpectedly accepted")
	}
	got, err := (AssetPath{Scope: ScopeExternalConfig, Absolute: t.TempDir(), Relative: "settings.json"}).ResolveExternal()
	if err != nil || !filepath.IsAbs(got) {
		t.Fatalf("ResolveExternal() = %q, %v", got, err)
	}
}

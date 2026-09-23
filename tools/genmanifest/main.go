// Command genmanifest builds the release-manifest.json and release-manifest.sig that
// cortex-ia's updater verifies before installing an update. It is a CI-only release
// tool: it never ships with the product, it never writes key material, and it reads the
// Ed25519 private key exclusively from the environment or a caller-provided file.
package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/updater"
)

const (
	defaultRepository = "lleontor705/cortex-ia"
	archiveNamePrefix = "cortex-ia"
	manifestFileName  = "release-manifest.json"
	signatureFileName = "release-manifest.sig"

	signingKeyEnv = "CORTEX_IA_RELEASE_SIGNING_KEY"
	keyIDEnv      = "CORTEX_IA_RELEASE_KEY_ID"
)

// Typed failures so a release job can tell a misconfigured key from a bad tag or a
// malformed dist tree. Every one of them aborts before any output file is written.
var (
	ErrMissingSigningKey = errors.New("release signing key unavailable")
	ErrNonCanonicalTag   = errors.New("non-canonical release tag")
	ErrMissingKeyID      = errors.New("release signing key ID unavailable")
	ErrNoArtifacts       = errors.New("no release archives found")
	ErrDuplicateArchive  = errors.New("duplicate release archive name")
	ErrUnsupportedTarget = errors.New("unsupported release target")
)

// The build matrix declared in .goreleaser.yaml; an unexpected target means the archive
// naming contract drifted and must not be signed blindly.
var (
	supportedOS   = map[string]bool{"windows": true, "linux": true, "darwin": true}
	supportedArch = map[string]bool{"amd64": true, "arm64": true}
)

type options struct {
	tag     string
	repo    string
	dist    string
	out     string
	keyID   string
	keyFile string
}

func main() {
	if err := run(os.Args[1:], os.Getenv); err != nil {
		fmt.Fprintf(os.Stderr, "genmanifest: %v\n", err)
		os.Exit(1)
	}
}

func run(argv []string, getenv func(string) string) error {
	flags := flag.NewFlagSet("genmanifest", flag.ContinueOnError)
	var opts options
	flags.StringVar(&opts.tag, "tag", "", "release tag bound into the manifest (for example v0.5.0)")
	flags.StringVar(&opts.repo, "repo", defaultRepository, "owner/repo the manifest is bound to")
	flags.StringVar(&opts.dist, "dist", "dist", "directory holding the built release archives")
	flags.StringVar(&opts.out, "out", "", "output directory for the manifest and signature (defaults to -dist)")
	flags.StringVar(&opts.keyID, "key-id", "", "trusted key ID echoed in the signature envelope")
	flags.StringVar(&opts.keyFile, "key-file", "", "file holding the base64 Ed25519 private key (overrides the environment)")
	if err := flags.Parse(argv); err != nil {
		return err
	}
	if opts.out == "" {
		opts.out = opts.dist
	}

	tag, err := canonicalTag(opts.tag)
	if err != nil {
		return err
	}
	privKey, err := loadPrivateKey(opts.keyFile, getenv)
	if err != nil {
		return err
	}
	keyID := strings.TrimSpace(opts.keyID)
	if keyID == "" {
		keyID = strings.TrimSpace(getenv(keyIDEnv))
	}
	if keyID == "" {
		return fmt.Errorf("%w: set -key-id or %s", ErrMissingKeyID, keyIDEnv)
	}
	artifacts, err := collectArtifacts(opts.dist, tag)
	if err != nil {
		return err
	}

	rawManifest, err := json.Marshal(updater.Manifest{
		SchemaVersion: updater.ManifestSchemaVersion,
		Repository:    opts.repo,
		Tag:           tag,
		Artifacts:     artifacts,
	})
	if err != nil {
		return fmt.Errorf("marshal release manifest: %w", err)
	}
	rawSig, err := updater.SignManifest(rawManifest, keyID, privKey)
	if err != nil {
		return fmt.Errorf("sign release manifest: %w", err)
	}

	if err := os.MkdirAll(opts.out, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := writeFileAtomic(filepath.Join(opts.out, manifestFileName), rawManifest); err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(opts.out, signatureFileName), rawSig)
}

func canonicalTag(raw string) (string, error) {
	tag := strings.TrimSpace(raw)
	if tag == "" {
		return "", fmt.Errorf("%w: tag is empty", ErrNonCanonicalTag)
	}
	if _, err := updater.ParseCanonicalVersion(tag); err != nil {
		return "", fmt.Errorf("%w: %w", ErrNonCanonicalTag, err)
	}
	return tag, nil
}

func loadPrivateKey(keyFile string, getenv func(string) string) (ed25519.PrivateKey, error) {
	var encoded string
	if keyFile != "" {
		raw, err := os.ReadFile(keyFile)
		if err != nil {
			return nil, fmt.Errorf("%w: read key file: %v", ErrMissingSigningKey, err)
		}
		encoded = string(raw)
	} else {
		encoded = getenv(signingKeyEnv)
	}

	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return nil, fmt.Errorf("%w: set %s or -key-file", ErrMissingSigningKey, signingKeyEnv)
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("%w: base64 decode failed", ErrMissingSigningKey)
	}
	if len(raw) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("%w: decoded key is %d bytes, want %d", ErrMissingSigningKey, len(raw), ed25519.PrivateKeySize)
	}
	return ed25519.PrivateKey(raw), nil
}

func collectArtifacts(distDir, tag string) ([]updater.ManifestArtifact, error) {
	version := strings.TrimPrefix(tag, "v")
	seen := make(map[string]bool)
	var artifacts []updater.ManifestArtifact

	err := filepath.WalkDir(distDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		targetOS, arch, ok := parseArchiveName(entry.Name(), version)
		if !ok {
			return nil
		}
		if !supportedOS[targetOS] || !supportedArch[arch] {
			return fmt.Errorf("%w: %s", ErrUnsupportedTarget, entry.Name())
		}
		if seen[entry.Name()] {
			return fmt.Errorf("%w: %s", ErrDuplicateArchive, entry.Name())
		}
		seen[entry.Name()] = true

		digest, size, err := hashFile(path)
		if err != nil {
			return err
		}
		artifacts = append(artifacts, updater.ManifestArtifact{
			Name:   entry.Name(),
			OS:     targetOS,
			Arch:   arch,
			Size:   size,
			SHA256: digest,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(artifacts) == 0 {
		return nil, fmt.Errorf("%w in %s", ErrNoArtifacts, distDir)
	}
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Name < artifacts[j].Name })
	return artifacts, nil
}

// parseArchiveName accepts exactly the updater's asset names:
// cortex-ia_<version>_<os>_<arch>.zip (windows) or .tar.gz (every other platform).
func parseArchiveName(name, version string) (string, string, bool) {
	lower := strings.ToLower(name)
	var ext string
	switch {
	case strings.HasSuffix(lower, ".tar.gz"):
		ext = ".tar.gz"
	case strings.HasSuffix(lower, ".zip"):
		ext = ".zip"
	default:
		return "", "", false
	}
	parts := strings.Split(name[:len(name)-len(ext)], "_")
	if len(parts) != 4 || parts[0] != archiveNamePrefix || parts[1] != version {
		return "", "", false
	}
	return parts[2], parts[3], true
}

func hashFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("open release archive %s: %w", filepath.Base(path), err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return "", 0, fmt.Errorf("hash release archive %s: %w", filepath.Base(path), err)
	}
	if size <= 0 || size > int64(updater.MaxArtifactSize) {
		return "", 0, fmt.Errorf("%w: %s has %d bytes", updater.ErrArtifactSizeInvalid, filepath.Base(path), size)
	}
	return hex.EncodeToString(h.Sum(nil)), size, nil
}

func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".genmanifest-*")
	if err != nil {
		return fmt.Errorf("create temporary file for %s: %w", filepath.Base(path), err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close %s: %w", filepath.Base(path), err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("set permissions on %s: %w", filepath.Base(path), err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("publish %s: %w", filepath.Base(path), err)
	}
	return nil
}

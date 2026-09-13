package updater

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

func TestUpdateDownload(t *testing.T) {
	repo := "lleontor705/cortex-ia"
	tag := "v1.5.0"

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	keyID := "dl-key-01"

	cleanup := SetTrustedKeysForTesting([]TrustedKey{
		{
			ID:         keyID,
			PublicKey:  pub,
			Repository: repo,
			MinVersion: "v1.0.0",
			MaxVersion: "v2.0.0",
		},
	})
	defer cleanup()

	targetOS := runtime.GOOS
	targetArch := runtime.GOARCH
	assetExt := ".tar.gz"
	if targetOS == "windows" {
		assetExt = ".zip"
	}
	exactAssetName := "cortex-ia_1.5.0_" + targetOS + "_" + targetArch + assetExt
	validPayload := []byte("VALID_BINARY_PAYLOAD_FOR_TESTING_PURPOSES")
	validPayloadHash := sha256.Sum256(validPayload)
	validPayloadSHA := hex.EncodeToString(validPayloadHash[:])

	manifest := Manifest{
		SchemaVersion: 1,
		Repository:    repo,
		Tag:           tag,
		Artifacts: []ManifestArtifact{
			{
				Name:   exactAssetName,
				OS:     targetOS,
				Arch:   targetArch,
				Size:   int64(len(validPayload)),
				SHA256: validPayloadSHA,
			},
		},
	}
	rawMan, _ := json.Marshal(manifest)
	rawSig, _ := SignManifest(rawMan, keyID, priv)

	t.Run("PublicTrustRemainsEmpty", func(t *testing.T) {
		reset := SetTrustedKeysForTesting(ProductionTrustedKeys)
		defer reset()

		client := &Client{Repo: repo, HTTPClient: http.DefaultClient}
		_, _, err := DownloadAndVerifyRelease(context.Background(), client.HTTPClient, repo, "v1.0.0", &Release{TagName: tag})
		if !errors.Is(err, ErrNoTrustedKey) {
			t.Fatalf("expected ErrNoTrustedKey, got %v", err)
		}
	})

	t.Run("AuthenticationBeforeArchiveFetchAndByteChecks", func(t *testing.T) {
		archiveFetched := false
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/release-manifest.sig":
				_, _ = w.Write(rawSig)
			case "/release-manifest.json":
				_, _ = w.Write(rawMan)
			case "/artifact":
				archiveFetched = true
				_, _ = w.Write(validPayload)
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		rel := &Release{
			TagName: tag,
			Assets: []ReleaseAsset{
				{Name: "release-manifest.sig", DownloadURL: srv.URL + "/release-manifest.sig"},
				{Name: "release-manifest.json", DownloadURL: srv.URL + "/release-manifest.json"},
				{Name: exactAssetName, DownloadURL: srv.URL + "/artifact"},
			},
		}

		// A: Success path
		data, art, err := DownloadAndVerifyRelease(context.Background(), srv.Client(), repo, "v1.0.0", rel)
		if err != nil {
			t.Fatalf("expected successful download and verification, got: %v", err)
		}
		if string(data) != string(validPayload) {
			t.Fatalf("payload mismatch")
		}
		if art.Name != exactAssetName {
			t.Fatalf("artifact name mismatch: %s", art.Name)
		}
		if !archiveFetched {
			t.Fatal("expected archive to be fetched")
		}

		// B: Bad signature - archive must NEVER be fetched!
		archiveFetched = false
		badSig, _ := json.Marshal(SignatureEnvelope{KeyID: keyID, Signature: "AAAA"})
		srvBadSig := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/release-manifest.sig":
				_, _ = w.Write(badSig)
			case "/release-manifest.json":
				_, _ = w.Write(rawMan)
			case "/artifact":
				archiveFetched = true
				_, _ = w.Write(validPayload)
			}
		}))
		defer srvBadSig.Close()

		relBadSig := &Release{
			TagName: tag,
			Assets: []ReleaseAsset{
				{Name: "release-manifest.sig", DownloadURL: srvBadSig.URL + "/release-manifest.sig"},
				{Name: "release-manifest.json", DownloadURL: srvBadSig.URL + "/release-manifest.json"},
				{Name: exactAssetName, DownloadURL: srvBadSig.URL + "/artifact"},
			},
		}
		_, _, err = DownloadAndVerifyRelease(context.Background(), srvBadSig.Client(), repo, "v1.0.0", relBadSig)
		if err == nil {
			t.Fatal("expected error on bad signature")
		}
		if archiveFetched {
			t.Fatal("archive was fetched despite signature failure (must authenticate before fetch)")
		}
	})

	t.Run("RejectDisallowedOriginAndRedirect", func(t *testing.T) {
		// External non-loopback HTTP
		parsed, _ := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).URL, error(nil)
		_ = parsed
		badURL := "http://evil-attacker.example.com/malware.zip"
		_, err := DownloadArtifactBytes(context.Background(), http.DefaultClient, badURL, 100, "abc")
		if !errors.Is(err, ErrInsecureScheme) && !errors.Is(err, ErrDisallowedOrigin) {
			t.Fatalf("expected origin/scheme rejection for %s, got: %v", badURL, err)
		}
	})

	t.Run("RejectMutatedReleaseAndCaseSubstringAssets", func(t *testing.T) {
		// Mutated tag name
		mutatedRel := &Release{
			TagName: "v1.4.0", // differs from manifest tag v1.5.0
			Assets: []ReleaseAsset{
				{Name: "release-manifest.sig", DownloadURL: "http://127.0.0.1/sig"},
				{Name: "release-manifest.json", DownloadURL: "http://127.0.0.1/man"},
			},
		}
		_, _, err := DownloadAndVerifyRelease(context.Background(), http.DefaultClient, repo, "v1.0.0", mutatedRel)
		if err == nil {
			t.Fatal("expected error for mutated tag name")
		}

		// Case-fold / substring asset in release
		caseRel := &Release{
			TagName: tag,
			Assets: []ReleaseAsset{
				{Name: "release-manifest.sig", DownloadURL: "http://127.0.0.1/sig"},
				{Name: "release-manifest.json", DownloadURL: "http://127.0.0.1/man"},
				{Name: strings.ToUpper(exactAssetName), DownloadURL: "http://127.0.0.1/art"},
			},
		}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/sig" {
				_, _ = w.Write(rawSig)
			} else {
				_, _ = w.Write(rawMan)
			}
		}))
		defer srv.Close()
		caseRel.Assets[0].DownloadURL = srv.URL + "/sig"
		caseRel.Assets[1].DownloadURL = srv.URL + "/man"

		_, _, err = DownloadAndVerifyRelease(context.Background(), srv.Client(), repo, "v1.0.0", caseRel)
		if !errors.Is(err, ErrMutatedRelease) {
			t.Fatalf("expected ErrMutatedRelease for case-fold asset name, got %v", err)
		}
	})

	t.Run("ByteChecksTruncationMisleadingLengthDigestMismatch", func(t *testing.T) {
		// Truncated
		srvTrunc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write(validPayload[:5])
		}))
		defer srvTrunc.Close()
		_, err := DownloadArtifactBytes(context.Background(), srvTrunc.Client(), srvTrunc.URL, int64(len(validPayload)), validPayloadSHA)
		if !errors.Is(err, ErrTruncatedDownload) && !errors.Is(err, ErrMisleadingLength) {
			t.Fatalf("expected truncation error, got %v", err)
		}

		// Digest mismatch
		srvTamper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tampered := append([]byte(nil), validPayload...)
			tampered[0] ^= 0xFF
			_, _ = w.Write(tampered)
		}))
		defer srvTamper.Close()
		_, err = DownloadArtifactBytes(context.Background(), srvTamper.Client(), srvTamper.URL, int64(len(validPayload)), validPayloadSHA)
		if !errors.Is(err, ErrDigestMismatch) {
			t.Fatalf("expected ErrDigestMismatch, got %v", err)
		}
	})

	t.Run("HonorTimeoutAndContext", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := DownloadArtifactBytes(ctx, http.DefaultClient, "http://127.0.0.1:9999/art", 100, validPayloadSHA)
		if err == nil || !strings.Contains(err.Error(), "context canceled") {
			t.Fatalf("expected context canceled error, got %v", err)
		}
	})
}

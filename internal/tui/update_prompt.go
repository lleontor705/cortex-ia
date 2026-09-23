package tui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/delegation"
	"github.com/lleontor705/cortex-ia/internal/updater"
)

const (
	inlineCheckEnv       = "CORTEX_IA_UPDATE_INLINE_CHECK"
	inlineCheckTimeout   = 3 * time.Second
	authorityGateTimeout = 5 * time.Second
	applyTimeout         = 10 * time.Minute
)

// Update seams exist so tests exercise the boot flow without downloading a real
// release, reaching the network, or touching the developer's delegation database.
var (
	applyUpdate       = applyUpdateToCurrentBinary
	probeAuthority    = probeActiveAuthority
	inlineUpdateCheck = checkLatestTag
)

// MaybePromptUpdate offers a cached update before the TUI takes over the
// terminal. It never blocks boot: a decline, EOF, a closed authority gate, or a
// typed apply failure leaves the running binary untouched and returns nil so the
// caller continues into the TUI. home is the user home; update state lives under
// home/.cortex-ia beside delegation.db.
func MaybePromptUpdate(home, version string, in *bufio.Reader, out io.Writer) error {
	if in == nil || out == nil {
		return errors.New("update prompt requires input and output streams")
	}
	if updater.IsDevOrUnknown(version) {
		return nil
	}

	stateRoot := filepath.Join(home, ".cortex-ia")
	state, err := updater.LoadUpdateState(stateRoot)
	if err != nil && !errors.Is(err, updater.ErrCorruptUpdateState) {
		return err
	}

	target := strings.TrimSpace(state.Available)
	if target == "" {
		target = optInInlineTarget(version)
	}
	if target == "" {
		return nil
	}

	_, _ = fmt.Fprintf(out, "Actualización disponible: %s (actual: %s).\n", target, version)
	_, _ = fmt.Fprintf(out, "¿Actualizar a %s? [y/N] ", target)
	answer, readErr := readAnswer(in)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return fmt.Errorf("read update confirmation: %w", readErr)
	}
	if !strings.EqualFold(answer, "y") {
		return nil
	}

	gateCtx, cancelGate := context.WithTimeout(context.Background(), authorityGateTimeout)
	defer cancelGate()
	active, gateErr := probeAuthority(gateCtx, delegation.DefaultDBPath(home))
	if gateErr != nil {
		var drift *delegation.SchemaDriftError
		if errors.As(gateErr, &drift) {
			_, _ = fmt.Fprintf(out, "Actualización bloqueada: %v\n", drift)
			_, _ = fmt.Fprintln(out, "Ejecuta 'cortex-ia update' en una consola para actualizar el binario antes de reintentar.")
			return nil
		}
		return fmt.Errorf("evaluate update authority gate: %w", gateErr)
	}
	if active {
		_, _ = fmt.Fprintf(out, "No se actualiza a %s: hay autoridad de trabajo activa (claims, leases o tareas in_progress).\n", target)
		_, _ = fmt.Fprintln(out, "La actualización queda disponible para el próximo arranque sin trabajo activo.")
		return nil
	}

	_, _ = fmt.Fprintf(out, "Descargando y verificando %s...\n", target)
	applyCtx, cancelApply := context.WithTimeout(context.Background(), applyTimeout)
	defer cancelApply()
	applied, applyErr := applyUpdate(applyCtx, version, strings.TrimSpace(state.AppliedFloor))
	if applyErr != nil {
		_, _ = fmt.Fprintf(out, "No se pudo aplicar la actualización: %v\n", applyErr)
		_, _ = fmt.Fprintln(out, "Se continúa con la versión actual instalada.")
		return nil
	}
	if strings.TrimSpace(applied) == "" {
		applied = target
	}
	if err := updater.RecordAppliedFloor(stateRoot, applied, ""); err != nil {
		_, _ = fmt.Fprintf(out, "La actualización se aplicó, pero no se pudo guardar el estado: %v\n", err)
		return nil
	}
	_, _ = fmt.Fprintf(out, "Actualización aplicada: reinicia cortex-ia para usar %s.\n", applied)
	return nil
}

func readAnswer(in *bufio.Reader) (string, error) {
	line, err := in.ReadString('\n')
	return strings.TrimSpace(line), err
}

// optInInlineTarget runs the explicitly opted-in inline check. The short timeout
// keeps an unreachable network from changing default boot latency; any failure
// degrades to silence rather than an error.
func optInInlineTarget(version string) string {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv(inlineCheckEnv)), "1") {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), inlineCheckTimeout)
	defer cancel()
	tag, err := inlineUpdateCheck(ctx, version)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(tag)
}

func checkLatestTag(ctx context.Context, version string) (string, error) {
	rel, hasUpdate, err := updater.New("").CheckLatest(ctx, version)
	if err != nil || !hasUpdate || rel == nil {
		return "", err
	}
	return rel.TagName, nil
}

// applyUpdateToCurrentBinary reuses the shipped download+verify+replace+rollback
// path. It deliberately avoids StateHome so floor persistence stays owned by
// MaybePromptUpdate, where a state-write failure is surfaced even though the
// binary was already replaced.
func applyUpdateToCurrentBinary(ctx context.Context, version, floor string) (string, error) {
	client := updater.New("")
	client.AppliedFloor = floor

	rel, hasUpdate, err := client.CheckLatest(ctx, version)
	if err != nil {
		return "", err
	}
	if !hasUpdate || rel == nil {
		return "", fmt.Errorf("no hay una release más reciente que %s", version)
	}

	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("no se pudo localizar el ejecutable actual: %w", err)
	}
	if err := client.ApplyUpdateToTarget(ctx, version, rel, execPath); err != nil {
		return "", err
	}
	return rel.TagName, nil
}

// probeActiveAuthority consults the global authority probe through a read-only
// handle. A missing database means no authority exists yet, so the gate stays
// open; a drifted schema fails closed before any handle is taken.
func probeActiveAuthority(ctx context.Context, dbPath string) (bool, error) {
	if err := delegation.CheckSchemaDriftBeforeOpen(dbPath); err != nil {
		return false, err
	}
	store, err := delegation.OpenStoreReadOnly(dbPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	defer func() { _ = store.Close() }()
	return delegation.HasAnyActiveAuthority(ctx, store)
}

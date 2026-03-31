#!/bin/bash
# Run E2E tests in Docker containers across multiple distros.
#
# Usage:
#   ./e2e/docker-test.sh              # Run on all distros
#   ./e2e/docker-test.sh ubuntu       # Run on specific distro
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
DISTROS="${1:-ubuntu fedora}"

echo "=== cortex-ia E2E Docker Tests ==="
echo "Project: $PROJECT_DIR"
echo "Distros: $DISTROS"
echo ""

PASS=0
FAIL=0

for distro in $DISTROS; do
    dockerfile="$SCRIPT_DIR/Dockerfile.$distro"
    if [ ! -f "$dockerfile" ]; then
        echo "[SKIP] No Dockerfile for $distro"
        continue
    fi

    echo "--- Building $distro ---"
    image="cortex-ia-e2e-$distro"
    docker build -t "$image" -f "$dockerfile" "$PROJECT_DIR" || {
        echo "[FAIL] Build failed for $distro"
        FAIL=$((FAIL+1))
        continue
    }

    echo "--- Running $distro ---"
    if docker run --rm "$image" -c "/src/e2e/e2e_test.sh"; then
        echo "[PASS] $distro"
        PASS=$((PASS+1))
    else
        echo "[FAIL] $distro"
        FAIL=$((FAIL+1))
    fi
    echo ""
done

echo "=== Docker E2E Summary ==="
echo "Passed: $PASS"
echo "Failed: $FAIL"

[ $FAIL -eq 0 ] || exit 1

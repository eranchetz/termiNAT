#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BIN_PATH="$ROOT_DIR/terminat-smoke"
INVALID_PROFILE="__terminat_smoke_invalid_profile__"
TMP_DIR="$(mktemp -d)"

cleanup() {
  rm -f "$BIN_PATH"
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

assert_contains() {
  local content="$1"
  local needle="$2"
  local label="$3"

  if [[ "$content" != *"$needle"* ]]; then
    echo "FAIL: $label"
    echo "Expected to find: $needle"
    echo "--- output ---"
    echo "$content"
    exit 1
  fi
}

echo "Running stream UI smoke checks..."
echo "Building binary..."
(
  cd "$ROOT_DIR"
  GOCACHE="${GOCACHE:-/tmp/go-build}" go build -o "$BIN_PATH" .
)

echo "Check: quick help exposes --ui"
quick_help="$("$BIN_PATH" scan quick --help 2>&1)"
assert_contains "$quick_help" "--ui" "quick help should include --ui flag"

echo "Check: deep help exposes --ui"
deep_help="$("$BIN_PATH" scan deep --help 2>&1)"
assert_contains "$deep_help" "--ui" "deep help should include --ui flag"

echo "Check: demo help exposes --ui"
demo_help="$("$BIN_PATH" scan demo --help 2>&1)"
assert_contains "$demo_help" "--ui" "demo help should include --ui flag"

echo "Check: quick rejects invalid --ui values"
set +e
"$BIN_PATH" scan quick --region us-east-1 --ui nope --profile "$INVALID_PROFILE" >"$TMP_DIR/quick-invalid-ui.out" 2>&1
status=$?
set -e
if [[ $status -eq 0 ]]; then
  echo "FAIL: quick invalid --ui should fail"
  cat "$TMP_DIR/quick-invalid-ui.out"
  exit 1
fi
assert_contains "$(cat "$TMP_DIR/quick-invalid-ui.out")" "invalid --ui value" "quick invalid ui error"

echo "Check: deep rejects invalid --ui values"
set +e
"$BIN_PATH" scan deep --region us-east-1 --duration 5 --ui nope --profile "$INVALID_PROFILE" >"$TMP_DIR/deep-invalid-ui.out" 2>&1
status=$?
set -e
if [[ $status -eq 0 ]]; then
  echo "FAIL: deep invalid --ui should fail"
  cat "$TMP_DIR/deep-invalid-ui.out"
  exit 1
fi
assert_contains "$(cat "$TMP_DIR/deep-invalid-ui.out")" "invalid --ui value" "deep invalid ui error"

echo "Check: demo rejects invalid --ui values"
set +e
"$BIN_PATH" scan demo --ui nope >"$TMP_DIR/demo-invalid-ui.out" 2>&1
status=$?
set -e
if [[ $status -eq 0 ]]; then
  echo "FAIL: demo invalid --ui should fail"
  cat "$TMP_DIR/demo-invalid-ui.out"
  exit 1
fi
assert_contains "$(cat "$TMP_DIR/demo-invalid-ui.out")" "invalid --ui value" "demo invalid ui error"

echo "Check: quick stream mode reaches scanner setup path"
set +e
"$BIN_PATH" scan quick --region us-east-1 --ui stream --profile "$INVALID_PROFILE" >"$TMP_DIR/quick-stream.out" 2>&1
status=$?
set -e
if [[ $status -eq 0 ]]; then
  echo "FAIL: quick stream smoke expected auth/profile failure with invalid profile"
  cat "$TMP_DIR/quick-stream.out"
  exit 1
fi
assert_contains "$(cat "$TMP_DIR/quick-stream.out")" "failed to create scanner" "quick stream scanner path"

echo "Check: deep stream mode reaches scanner setup path"
set +e
"$BIN_PATH" scan deep --region us-east-1 --duration 5 --ui stream --auto-approve --profile "$INVALID_PROFILE" >"$TMP_DIR/deep-stream.out" 2>&1
status=$?
set -e
if [[ $status -eq 0 ]]; then
  echo "FAIL: deep stream smoke expected auth/profile failure with invalid profile"
  cat "$TMP_DIR/deep-stream.out"
  exit 1
fi
assert_contains "$(cat "$TMP_DIR/deep-stream.out")" "failed to create scanner" "deep stream scanner path"

echo "Check: demo stream mode renders serial report"
demo_stream="$("$BIN_PATH" scan demo --ui stream 2>&1)"
assert_contains "$demo_stream" "Deep Dive Scan Complete" "demo stream should render report"

echo "PASS: stream UI smoke checks completed"

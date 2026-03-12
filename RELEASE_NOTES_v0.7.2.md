# Release Notes - v0.7.2

Release date: 2026-03-12

## Summary

v0.7.2 is a bug-fix release addressing issues found during E2E testing. No breaking changes.

---

## Issues fixed

### Issue #37 — Approval prompt hardcodes "5 startup" minutes

The deep scan approval prompt previously said:

```
Total scan time estimate: 10 minutes (5 startup + 5 collection)
```

Flow Logs startup actually polls until ACTIVE (typically 1-3 min, up to 5 min). The fixed prompt reads:

```
Total scan time estimate: up to 10 minutes (~5 startup + 5 collection)
```

Both stream mode (`ui/deep_scan_stream.go`) and TUI mode (`ui/deep_scan.go`) are updated.

---

### Issue #36 — Traffic generation script problems

Multiple problems in the `test/infrastructure/test-stack.yaml` EC2 UserData script:

| Problem | Fix |
|---------|-----|
| IMDSv1 `curl` fails on modern EC2 defaults (`HttpTokens: required`) | Switched to IMDSv2 token-based metadata access |
| CloudFormation `sed` placeholder injection broke due to embedded newlines in YAML block scalars | Removed sed/placeholder approach entirely; script requires positional args |
| S3 test file was 1MB (single file) | Lambda now creates 10 × 1MB files (`test-file-0.bin` … `test-file-9.bin`) |
| Nested `aws s3 cp` loop (500 sequential CLI calls) couldn't complete in 5 min | Replaced with `aws s3 sync` — one process, all files, ~10× faster |
| `FROM public.ecr.aws/lambda/python:3.12` pulled ~500MB through NAT, classified as "Other" | Switched to `FROM scratch` — no base image pull, only 20MB random layer pushed to private ECR |
| DynamoDB scans used `--max-items 10` (tiny traffic) | Removed limit; full table scan (100 items × 1KB payload ≈ 100KB/scan) |
| `sleep 10` between batches limited throughput | Reduced to `sleep 2` |
| ECR push layer was 5MB | Increased to 20MB |

---

## Other fixes

### CloudWatch log group not deleted after scan

After stopping Flow Logs, the tool immediately called `DeleteLogGroup`. CloudWatch sometimes defers the actual deletion while flushing in-flight buffered writes, causing the log group to persist silently even though the API returned success.

**Fix**: Added a 15-second context-cancellable wait between `stopFlowLogs()` and the log group deletion, giving CloudWatch time to drain.

### Orphaned log groups accumulating in test account

`cleanup.sh` deleted the CloudFormation stack but never cleaned up CloudWatch log groups created dynamically by `terminat` scans. After multiple test runs, these accumulated indefinitely.

**Fix**: `cleanup.sh` now deletes all `/aws/vpc/flowlogs/terminat*` log groups in the region before stack deletion.

### `continuous-traffic.sh` silent failure on missing stack outputs

If the CloudFormation stack query returned empty values, the SSM command was sent with blank arguments, causing the traffic script to fail silently.

**Fix**: Added validation that all four outputs (instance ID, bucket, table, repo URI) are non-empty before dispatching the command.

---

## Internal / code quality

- Fixed 17 instances of `\n` embedded inside `lipgloss.Style.Render()` calls in `ui/deep_scan.go` and `ui/quick_scan.go` (violates the project lipgloss convention; newlines must be appended after `Render()`).
- `test/TESTING.md` updated: correct binary name (`terminat`), current script names (`continuous-traffic.sh`), valid CLI flags (removed nonexistent `--nat-gateway-ids`, `--output`), correct S3 file names.
- Lambda population function timeout increased from 60s to 120s.
- EC2 instance `MetadataOptions` set to `HttpTokens: optional` (allows both IMDSv1 and v2).

---

## Upgrade notes

No CLI or API changes. Drop-in replacement for v0.7.1.

If you maintain a fork of `test-stack.yaml`: the EC2 UserData no longer uses sed to inject resource names. The `generate-traffic.sh` script now requires all 6 positional arguments (`<duration> <s3_batches> <ddb_requests> <bucket> <table> <repo>`). `continuous-traffic.sh` passes them automatically from CloudFormation outputs.

---

## Verification

```
go test ./...       ✅
gofmt -l .          ✅ (no output)
go vet ./...        ✅
smoke-ui-stream.sh  ✅ (9/9 checks)
E2E (full run)      ✅ (stack deploy → traffic → deep scan → cleanup)
```

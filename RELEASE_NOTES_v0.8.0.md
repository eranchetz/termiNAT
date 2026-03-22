# Release Notes - v0.8.0

Release date: 2026-03-22

## Summary

v0.8.0 is a stabilization and release-operations update. It focuses on making scans easier to target, keeping stream/TUI/report output on one canonical model, and cleaning up the docs and release workflow so the tool is easier to run, review, and maintain.

No breaking CLI changes are introduced.

---

## Highlights

### Region-scoped target selection

- `scan quick` and `scan deep` now support explicit target narrowing inside a single AWS region.
- Use `--vpc-id` for one VPC, `--vpc-ids` for multiple VPCs, and `--nat-gateway-ids` for specific NAT Gateways.
- If no target filters are supplied, the scan defaults to all NAT Gateways discovered in the selected region.

### Canonical report and export model

- Stream output, TUI rendering, markdown export, and DataHub events now share the same report data model.
- Per-VPC endpoint analysis is preserved across rendering and export paths so multi-VPC runs stay consistent.
- Summary output now includes findings, remediation commands, and cost context in one place.

### Docs and release workflow cleanup

- The quick-start docs now lead with the simplest operating model: one region per run, then narrow to the VPCs or NAT Gateways you care about.
- The testing guide and playground E2E script now point to the current smoke and validation workflow.
- The release script now prefers curated release notes when present, so GitHub releases can ship with reviewed release text instead of ad hoc generated notes.

---

## Verification

- `go test ./...` ✅
- `go vet ./...` ✅
- `git diff --check` ✅
- `./test/scripts/smoke-ui-stream.sh` ✅
- `AWS_PROFILE=chetz-playground AWS_REGION=us-east-1 ./test/scripts/run-e2e-test.sh` ✅

---

## Upgrade notes

- Drop-in upgrade from `v0.7.2`.
- Release artifacts are published with the `terminat` binary name.
- The release workflow now uses `RELEASE_NOTES_v0.8.0.md` when creating the GitHub release body.

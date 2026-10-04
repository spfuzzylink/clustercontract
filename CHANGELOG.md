# Changelog

## v0.1.0-alpha.1 — 2026-10-04 (experimental pre-alpha)

- Add scope-specific acceptance criteria, environment/unit/freshness/sample checks, and visible expiring waivers.
- Preserve missing and incompatible evidence as incomplete; prevent older passing observations from hiding newer failures.
- Add deterministic JSON/Markdown reports, a Go API and CLI exit-code contracts.
- Add a narrow NCCL all_reduce_perf text adapter that separates message size and placement and preserves correctness failures.
- Include synthetic demonstrations, validation tests and an explicit integration/measurement boundary.

No NVIDIA hardware validation, customer acceptance certification, or production use is claimed.

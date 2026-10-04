Experimental first release: customer acceptance requirements evaluated against recorded benchmark evidence.

- Go library and CLI with deterministic JSON/Markdown reports.
- Per-scope checks for thresholds, environment, units, freshness and sample coverage; explicit expiring waivers.
- Narrow NCCL all_reduce_perf text importer that preserves message-size/placement distinctions and correctness failures.
- Runnable synthetic examples and tests for missing, stale, inconsistent and failed evidence.

**Validation boundary:** synthetic fixtures and local software tests only. No NVIDIA GPU performance, hardware certification, operator adoption or production readiness is claimed. Owners, environment identity and waivers are caller-supplied metadata.

Download the archive for your OS/architecture, verify it against SHA256SUMS and extract it. Archives include the binary and examples. For example:

```sh
./clustercontract version
./clustercontract evaluate --contract examples/contract.json --evidence examples/pass.json --at 2026-10-04T12:00:00Z
```

Linux and macOS builds are provided for amd64/arm64. The macOS arm64 package was executed locally; other archives were cross-compiled. See the README for supported input format and limitations. Checksums are not signatures.

Apache-2.0. Independent project, not affiliated with or endorsed by NVIDIA.

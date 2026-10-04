# ClusterContract

**Customer acceptance contracts for AI compute infrastructure.**

A benchmark can finish successfully while a customer handover still lacks evidence for a required node, uses results from a previous configuration, or depends on an expired exception. ClusterContract records who owns each obligation, evaluates evidence for every required scope, and makes exceptions explicit in a reproducible acceptance report.

**Pre-alpha. All included examples are SYNTHETIC NOT GPU VALIDATION.** No GPU, network, storage, or cluster performance has been measured by this project. The current release is a local Go library and CLI, with a normalized JSON input and a narrow NCCL text adapter. Its product hypothesis is that customer-owned, cross-tool obligations and expiring waivers can make handovers easier to review. Customer value and differentiation still require validation.

## Build and run

Requires Go 1.25 or newer. Standard library only; no runtime service or GPU is required to evaluate saved evidence.

```sh
go build -o bin/clustercontract ./cmd/clustercontract
./bin/clustercontract version
./bin/clustercontract evaluate \
  --contract examples/contract.json \
  --evidence examples/pass.json \
  --at 2026-10-04T12:00:00Z
```

Use `--format json` for automation. Markdown is the default human report. Reports go to stdout and invalid-input errors go to stderr. The evaluation clock is always explicit; reports do not change merely because a command is run on another day. The examples use invented thresholds, not recommendations for any hardware.

Run `sh scripts/demo.sh` for an end-to-end demonstration with asserted CLI exit codes: raw NCCL-format text becomes evidence; a communicator contract passes; a broader customer handover stays incomplete because the second communicator and storage coverage are missing; and a high-bandwidth run with a correctness error fails. Each report shows the affected obligation, owner, and selected evidence source. The final example demonstrates a conditional waiver. All data is synthetic. Set `CLUSTERCONTRACT_GO` to a Go binary path if `go` is not on your PATH.

| Example | Contract | Evidence | Verdict | Exit |
|---|---|---|---|---:|
| Complete coverage | `contract.json` | `pass.json` | pass | 0 |
| One slow node despite a healthy overall average | `contract.json` | `fail.json` | fail | 1 |
| One unmeasured node | `contract.json` | `incomplete.json` | incomplete | 1 |
| Explicit exception for the unmeasured node | `waiver-contract.json` | `incomplete.json` | conditional | 3 |
| Malformed input, unsupported format, or output failure | any | any | no acceptance report | 2 |

Run each example with the fixed time above. `conditional` is deliberately a separate nonzero exit code: automation must explicitly decide whether that outcome is acceptable. `go run` wraps nonzero program exits; build the binary when using these exit codes in a pipeline.

## Release archives

The [GitHub releases](https://github.com/spfuzzylink/clustercontract/releases) provide experimental binaries for Linux and macOS on amd64 and arm64, with SHA-256 checksums. Each archive includes the executable, license, documentation and synthetic examples. After extracting the archive, run:

```sh
./clustercontract version
./clustercontract evaluate --contract examples/contract.json --evidence examples/pass.json --at 2026-10-04T12:00:00Z
```

The source checkout uses `./bin/clustercontract`; an extracted release uses `./clustercontract`. No Go installation is needed for a prebuilt binary. Verify the archive against `SHA256SUMS` before use. Checksums detect differing bytes; they are not a code signature.

## Evaluation rules

- A criterion expands into one obligation for **each** explicitly named scope. Node results are never pooled; there is no inferred coverage of unmeasured nodes.
- The newest evidence record for a `(scope, metric)` is selected. A newer failed, short, stale, future-dated, or incompatible record prevents fallback to an older passing record. Equal timestamps for that pair are invalid rather than resolved by input order.
- Unit and environment fingerprint must match exactly. There is no implicit unit conversion. An environment fingerprint is an opaque producer-supplied string; include the hardware inventory, firmware/software versions, topology, relevant configuration, and measurement protocol in whatever digest your evidence producer supplies. ClusterContract does not independently inspect that environment.
- Every sample must meet the inclusive minimum/maximum bounds, and the selected record must contain at least `min_samples`. Observations from different records are not combined to reach that count. This is a worst-sample rule, not a percentile or statistical confidence test.
- Evidence is current when `recorded_at <= evaluation_time` and age is at most `max_age_seconds`. Missing, stale, future, incompatible, or insufficient evidence is incomplete. A known correctness error fails the obligation, including when coverage is also incomplete. Unknown correctness is incomplete when `require_zero_errors` is set.
- A waiver names one exact criterion/scope, accountable owner, reason, and expiry. It applies only before expiry and only to an obligation that otherwise fails or is incomplete. The underlying result remains visible. A passing obligation keeps a visible `not_needed` waiver; expired waivers remain visible and do not apply.
- Contract verdict precedence is `fail`, then `incomplete`, then `conditional` if any obligation was waived, otherwise `pass`. Waiving one node cannot hide an unwaived failure elsewhere.
- Surplus evidence is permitted but its IDs appear as unused. Schema violations and duplicate identifiers, scopes, or waivers fail validation. Unknown JSON fields, case-altered field names, duplicate keys, null values, non-finite numbers, and trailing JSON are rejected. JSON inputs are limited to 16 MiB and 64 nesting levels.
- Local resource limits are 50,000 expanded obligations, 100,000 evidence records, 1,000,000 total samples and 2,000,000 selected-sample comparisons per evaluation. Nonempty metadata fields are at most 1,024 bytes; descriptions are at most 4,096 bytes. The accumulated compact JSON obligation payload is capped at 64 MiB (formatted output can be larger). Oversized work is invalid, never a partial acceptance. These are defensive ceilings, not tested production-scale claims.

## Complete v1 input schema

JSON fields are case sensitive. Optional fields should be omitted, not set to null. Identifiers, scopes, owners, units, fingerprints, provenance source/reference, and waiver reasons must be nonempty without surrounding whitespace or newline/tab separators. RFC3339 timestamps include a timezone. Required scalar fields that have no sensible default must be supplied; `require_zero_errors` defaults to false.

| Contract field | Type | Required | Meaning |
|---|---|---|---|
| `version` | string | yes | Exactly `v1` |
| `name` | string | yes | Human contract name |
| `description` | string | no | Context propagated to reports |
| `environment_fingerprint` | string | yes | Exact expected environment identity |
| `criteria` | array | yes | Nonempty list of criteria below |
| `waivers` | array | no | Scoped exceptions below |

| Criterion field | Type | Required | Meaning |
|---|---|---|---|
| `id` | string | yes | Unique within the contract |
| `owner` | string | yes | Team or person accountable for this obligation |
| `metric` | string | yes | Exact evidence metric identity, including protocol dimensions |
| `unit` | string | yes | Exact unit, such as `GB/s` |
| `scopes` | string array | yes | Nonempty unique node IDs or other explicit scope IDs |
| `minimum`, `maximum` | finite number | at least one | Inclusive bounds for every sample |
| `min_samples` | positive integer | yes | Samples required in one selected record |
| `max_age_seconds` | positive integer | yes | Maximum evidence age; maximum 9223372036 seconds |
| `require_zero_errors` | boolean | no | Require a reported correctness count of zero |

| Waiver field | Type | Required | Meaning |
|---|---|---|---|
| `criterion_id`, `scope` | string | yes | One existing obligation; no wildcards |
| `owner`, `reason` | string | yes | Accountability and explanation |
| `expires_at` | RFC3339 string | yes | Exclusive expiry |

Evidence bundles have `version: "v1"`, optional `description`, and a required `evidence` array, which may be empty.

| Evidence field | Type | Required | Meaning |
|---|---|---|---|
| `id` | string | yes | Unique record identifier |
| `recorded_at` | RFC3339 string | yes | Observation completion time |
| `environment_fingerprint` | string | yes | Producer-supplied environment identity |
| `scope`, `metric`, `unit` | string | yes | Exact dimensions matched against the contract |
| `samples` | finite number array | yes | Nonempty observations from the same protocol and environment |
| `correctness_errors` | unsigned integer | no | Missing means unknown; any reported positive value fails |
| `provenance` | object | yes | Required `source` and `reference` strings; optional `sha256` |

`sha256`, when provided, must be 64 lowercase hexadecimal characters. The evaluator checks its format, not whether a referenced file exists or matches it. The NCCL adapter computes it from the exact input bytes. A provenance reference is displayed as text and never fetched or executed.

## NCCL all_reduce_perf adapter

```sh
./bin/clustercontract import-nccl \
  --input examples/nccl-synthetic.log \
  --scope communicator/rack-a \
  --environment synthetic-env-v1 \
  --recorded-at 2026-10-04T11:45:00Z \
  --reference examples/nccl-synthetic.log \
  --description 'SYNTHETIC NOT GPU VALIDATION' > /tmp/clustercontract-evidence.json

./bin/clustercontract evaluate \
  --contract examples/nccl-contract.json \
  --evidence /tmp/clustercontract-evidence.json \
  --at 2026-10-04T12:00:00Z --format json
```

The parser implements the 13-column text layout in NVIDIA's [nccl-tests v2.13.10 source](https://github.com/NVIDIA/nccl-tests/blob/d2d40cc8249378efa4d7e2c949528c15eeb7d8e7/src/common.cu) and the [all-reduce operation](https://github.com/NVIDIA/nccl-tests/blob/d2d40cc8249378efa4d7e2c949528c15eeb7d8e7/src/all_reduce.cu). The numeric table contains five operation fields followed by timing, algorithm bandwidth, bus bandwidth, and correctness for each placement. The adapter uses **bus bandwidth in decimal GB/s**, preserving separate out-of-place and in-place metrics. Message size, datatype, and reduction operation are also encoded in each metric name; they are never averaged together.

Supported input requires placement, column and unit headers, complete numeric rows, and the final correctness summary. It rejects MPI-prefixed rows, interleaved debug output, CPU-time headers, concatenated runs, unknown layouts, and incomplete logs. Supported datatypes are int8/uint8/int32/uint32/int64/uint64/half/float/double/bfloat16. All-reduce rows require root `-1` and consistent size/count/type. This is deliberately a narrow parser, not a universal NCCL log importer.

Each printed row is one aggregate observation. Benchmark internal iterations are **not** counted as independent samples. Repeated rows of the same size/type/operation contribute samples in the same record. `N/A` correctness remains unknown; a nonzero run summary conservatively propagates a known failure to every emitted metric, so selecting one clean size or placement cannot hide a failed run. The imported count is a conservative failure indicator when run-level and row-level counts differ, not an exact total of corrupted elements. A row failure with a zero summary is rejected as inconsistent input. Import success means conversion succeeded, not that acceptance passed.

The scope must represent the entire tested communicator. Collective bandwidth is not per-node evidence. The timestamp, environment, reference, communicator scope, and assertion that the log is from `all_reduce_perf` come from the caller; the adapter cannot authenticate them from text. For real use, preserve the command, executable version, inventory, return code, stdout/stderr, measurement protocol, and environment digest outside this CLI. A completed text footer cannot by itself prove successful process termination.

## Library

The public API is `DecodeContract`, `DecodeEvidence`, `Evaluate`, `WriteMarkdown`, and `ImportNCCL`. Both decoders and `Evaluate` validate inputs, so programmatic callers cannot bypass the core validation accidentally.

```go
report, err := clustercontract.Evaluate(contract, evidence, evaluationTime)
if err != nil {
    return err // invalid input; no acceptance decision
}
fmt.Println(report.Verdict, report.ExitCode())
```

Reports retain thresholds, selected record identity, timestamps, sample count/range, provenance, reasons, and waiver details. Obligation order is canonical by criterion/scope; unused IDs are sorted; all output timestamps are UTC. Reports do not consult the network, referenced files, or current wall clock. See [architecture](docs/architecture.md).

## Tests and release limits

```sh
go test -race ./...
go vet ./...
```

Tests cover healthy averages hiding a bad node/sample, missing and stale coverage, future timestamps, incompatible environments/units, newest-record regressions, duplicates/malformed JSON, correctness failures, waived/expired obligations, deterministic ordering, NCCL placement separation, incomplete logs, and CLI exits. Fixtures test software decision behavior only.

This project does not run benchmarks, tune NVIDIA software, replace NVCRE/AICR/ReFrame/DCGM, schedule jobs, discover infrastructure, sign evidence, manage approval identities, or provide an acceptance certification. No live NVCRE, AICR, ReFrame, Kubernetes, Slurm, or DCGM integration is implemented. The input model can represent evidence produced by those tools, but that is not a claim of integration. Owners and waivers are plain metadata without access control; review contract changes in your existing approval process.

Before operational adoption: validate real single-node and multi-node runs; replay known corruption and transport failures; compare decisions against a jointly agreed customer acceptance plan; validate provenance capture and inventory coverage; exercise drift and waiver expiry; and obtain independent review. Planned work includes versioned evidence producers for existing assessment tools, authenticated provenance, measurement-protocol schemas, and change-to-handover workflows. These are roadmap items, not shipped capabilities.

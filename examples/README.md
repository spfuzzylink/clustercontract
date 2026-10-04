# Synthetic decision examples

**Every file here is SYNTHETIC NOT GPU VALIDATION.** Values, identities and thresholds were invented to exercise the evaluator. No hardware benchmark was executed. The NCCL log demonstrates the official column layout with invented values.

Use evaluation time `2026-10-04T12:00:00Z`.

- `contract.json` + `pass.json`: both explicitly required nodes meet every sample bound.
- `contract.json` + `fail.json`: node-b fails although averaging it with node-a would conceal the regression.
- `contract.json` + `incomplete.json`: node-b is unmeasured and cannot pass.
- `waiver-contract.json` + `incomplete.json`: node-b has a visible, expiring exception; verdict conditional and exit 3.
- `nccl-synthetic.log` converted by `import-nccl` + `nccl-contract.json`: separate collective scopes and in-place/out-of-place bus-bandwidth metrics. These are aggregate communicator observations, never per-node coverage.
- That same imported evidence + `cross-tool-contract.json`: the healthy rack-a collective cannot satisfy missing rack-b or node-level storage obligations; verdict incomplete.
- `nccl-correctness-failure.log` converted by `import-nccl` + `nccl-contract.json`: high bus bandwidth with a correctness error; verdict fail. A run-level error conservatively affects both placements.

The waiver expires at `2026-10-05T12:00:00Z`. Evaluation at that instant cannot apply it; the evidence will also be stale, deliberately leaving the contract incomplete.

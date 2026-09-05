# ADR 0020 — Deterministic extra built-ins: round(x,n), abs, trunc, modulo

- **Status**: Accepted (2026-06-24)
- **Deciders**: santosh-sankranthi

## Context

[ADR 0004 §3](0004-deferrals.md) deferred **all** multi-argument built-ins with a blanket rule, and
the single-arg family was limited to `floor` / `ceiling` / `round(x)`. A source-level gap analysis
against the major rule engines (json-rules-engine, json-logic, GoRules ZEN, node-rules, DMN/FEEL —
see [comparison.md](../comparison.md)) found that a handful of those excluded functions are
**near-universal** in real pricing/tax/scoring rules and are **fully deterministic, exact, and
verification-safe** — they were excluded only by the blanket rule, not by feelc's philosophy:

- `round(x, n)` — round money to `n` decimal places (the `round(x*100)/100` workaround is error-prone);
- `abs(x)` — tolerance/variance rules (`|actual − target| <= tol`);
- `trunc(x)` — truncate toward zero (completes the rounding family);
- `modulo(x, y)` — parity / cyclic-bucketing rules.

## Decision

Whitelist these four deterministic built-ins (a narrow carve-out of ADR 0004 §3); everything else
multi-arg (`substring`, string/list functions, `for`/`some`/`every`) still **fails outright**.

- **`abs(x)`, `trunc(x)`** — single-arg, added to the built-in table (`internal/compiler/lower_expr.go`),
  new opcodes `OpAbs` / `OpTrunc` (`trunc` = floor for x ≥ 0, ceil for x < 0).
- **`round(x, n)`** — two-arg, round to `n` decimal places, HALF_EVEN (`OpRoundN`, via `apd.Quantize`
  on the frozen context). `round(x)` (one-arg, to the nearest integer) is unchanged. Non-integer or
  out-of-range `n` is a runtime error.
- **`modulo(x, y)`** — two-arg, **DMN floored modulo** `x − y·floor(x/y)` (the result follows the
  divisor's sign), `OpMod`. Modulo by zero is a runtime error. Chosen as the **FEEL-standard function
  form** (`modulo(a, b)`) rather than a `mod` keyword, so the vendored FEEL parser is untouched and
  `%` stays the percent literal.

`null` and non-applicable (`TagNA`) propagate through all four. Units: `round`/`abs`/`trunc` preserve
the operand's unit; `modulo` requires the operands to share a dimension (like `+`/`−`).

## Consequences

- Closes the top "philosophy-compatible gaps" from [comparison.md](../comparison.md) **without**
  weakening any guarantee: every result stays bit-for-bit exact under the frozen HALF_EVEN context
  (ADR 0002) and deterministic.
- **Verification:** these appear in literal-expression decisions / `Op=Prog` cells, which are already
  non-geometric; the SMT backend degrades them to `not-verifiable` (sound honest fallback) — the
  geometric completeness/conflict proofs are unaffected.
- **Test contract:** the conformance tripwire tests for these features flip from *rejected* →
  *supported* (`packages/engine/test/conformance.test.ts`); new Go tests cover the arithmetic and the
  error paths (`internal/engine/feel_ext_test.go`). The remaining gaps (bounded quantifiers,
  `starts_with`/`contains` cell tests, expression-level Kleene, context-field read) stay deferred with
  live tripwires.
- ADR 0004 §3 is amended (append-only) to point here.

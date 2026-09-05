# feelc structured error schema

Compilation errors from parsing, type-checking, and lowering `.rules` files are
positioned objects. With `--json`, `feelc run`, `verify`, and `check` emit them
on stdout so the authoring loop can use `line`, `col`, and `suggestion`.

## JSON format

```json
{
  "file": "credit.rules",
  "line": 12,
  "col": 7,
  "code": "DSL002",
  "message": "invalid FEEL cell",
  "suggestion": "..."
}
```

`file`, `col`, `code`, and `suggestion` are omitted when unknown or empty.
`line` and `message` are always present; an unknown line is `0`. Columns are
one-based and reliable for table cells. For literal expressions and single-line
declarations, only the line is guaranteed.

## Stable code catalog

### DSL parser

| Code | Meaning |
|---|---|
| `DSL001` | unrecognized instruction |
| `DSL002` | invalid FEEL cell or expression |
| `DSL003` | missing `model` declaration |
| `DSL004` | malformed `model` header |
| `DSL005` | malformed input |
| `DSL006` | malformed decision header |
| `DSL007` | unrecognized decision body line |
| `DSL008` | malformed rule; `=>` missing |
| `DSL009` | empty cell |
| `DSL010` | malformed type declaration |
| `DSL011` | unsupported type |
| `DSL012` | content after `{` on the header line |

### Compiler and type-checker

| Code | Meaning |
|---|---|
| `CMP001` | reference to an undeclared name |
| `CMP002` | unsupported hit policy |
| `CMP003` | unknown decision type |
| `CMP004` | wrong number of conditions or outputs |
| `CMP005` | PRIORITY constraint not satisfied |
| `CMP006` | COLLECT constraint not satisfied |
| `CMP007` | construct outside the supported subset |
| `CMP008` | literal expected |

Text errors remain backward-compatible and omit suggestions. Verification
findings and semantic-check verdicts have separate JSON forms.

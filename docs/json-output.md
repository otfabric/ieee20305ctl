# JSON Output

All commands that produce output support the `--output json` flag (or `-o json`).
When enabled, the command writes a single JSON object to stdout with the following
envelope:

```json
{
  "status": "ok",
  "data": { ... }
}
```

On error:

```json
{
  "status": "error",
  "error": "description of the error"
}
```

This makes ieee20305ctl suitable for scripting and integration with other tools
via `jq` or similar JSON processors.

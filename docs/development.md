# Development

Working on the `nats-chat` MCP server (the TypeScript package at the repo root).
For the Go console see [console/README.md](../console/README.md).

```bash
npm run build         # compile TypeScript to dist/
npm run typecheck     # type-check without emitting
npm run dev           # run from source via tsx
npm run test          # integration tests (requires a running Docker daemon)
```

## Integration tests

Integration tests use [Testcontainers](https://testcontainers.com/) to boot a
throwaway JetStream-enabled NATS broker and exercise the real publish / consume /
KV / history paths — so a Docker daemon must be reachable. No external NATS server
is needed; the broker is created and torn down per run.

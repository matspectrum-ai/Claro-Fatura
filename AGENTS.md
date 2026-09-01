# Claro Fatura migration instructions

This repository is a behavior-preserving rewrite of the existing Claro Fatura application.

## Hard constraints

- Do not redesign the product or add features unless explicitly requested.
- Preserve current routes, visible text, workflows, database schema, CSV/XLSX import semantics, gateways, payment statuses and admin behavior.
- Treat the supplied 2026-09-01 application snapshot as the behavioral oracle.
- Prefer Go standard library and browser-native APIs. Add dependencies only when they remove more complexity than they introduce.
- Do not introduce microservices, Redis, message brokers or Kubernetes without measured evidence that the simpler architecture fails a stated requirement.
- Never expose `SUPABASE_SERVICE_ROLE_KEY` or gateway secrets to browser code.
- Every migrated business rule needs characterization tests.
- Payment mutations must remain idempotent and webhook handling must be safe under duplicate delivery.
- Verify with `go test ./...` before committing Go changes.

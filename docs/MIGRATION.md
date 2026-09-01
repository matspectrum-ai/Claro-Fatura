# Behavior-preserving migration

## Objective

Reimplement the existing Claro Fatura product on a lighter runtime without changing the product's visible behavior, database model, business rules, payment providers, administration capabilities, or import workflow.

## Target stack

- Public and admin UI: HTML + CSS + browser JavaScript.
- Backend: Go using the standard library first.
- Database/Auth: the existing Supabase project and existing schema.
- Deployment target: a single Linux VPS behind a reverse proxy. Horizontal replication remains possible without changing application semantics.

## Non-goals

- No redesign.
- No new product features.
- No new database product.
- No microservices.
- No Kubernetes.
- No Redis/queue unless load tests demonstrate a concrete bottleneck requiring it.

## Compatibility rules

The snapshot supplied on 2026-09-01 is the behavioral oracle. The rewrite must preserve:

1. Phone-only public lookup.
2. Phone normalization variants with/without Brazil country code and ninth digit.
3. Only the latest payable invoice whose due date is inside the current UTC month.
4. The same payable statuses: `em_aberto`, `vencida`, `em_processamento`, `falhou`, `expirada`.
5. Access logging must never make a customer lookup fail.
6. PIX value must use `valor_desconto` exactly as the current implementation does.
7. Existing gateway routing, idempotency, webhook confirmation, transaction/payment persistence and admin behavior must be ported before cutover.
8. Existing Supabase tables and data remain authoritative.

## Verification gate

A module is considered migrated only when its characterization tests pass against the preserved behavior. Performance claims require load-test evidence; language choice alone is not evidence of capacity.

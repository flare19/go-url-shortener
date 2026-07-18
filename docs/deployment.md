# Deployment

## Local

```bash
docker-compose up
```

_(fill in once docker-compose.yml exists — services, ports, env vars)_

## Environment variables

| Variable      | Service     | Description               | Default |
|---------------|-------------|----------------------------|---------|
| `MONGO_URI`   | both        | Mongo connection string    |         |
| `WRITER_PORT` | writer      |                             |         |
| `REDIRECT_PORT`| redirector |                             |         |

# go-url-shortener — Deployment Spec

Status: decision finalized (2026-07-18), infra not yet built.
This doc exists so the hosting decision and reasoning survive across sessions — read cold if needed.

---

## 1. Hosting Decision

**Chosen: Azure for Students, two separate VMs (one per service) — writer on B1s,
redirector on B2ats v2 — no reverse-proxy VM, each service directly reachable.**

### Why not a fresh AWS account
AWS changed its free tier on July 15, 2025. New accounts no longer get the old 12-month
free-usage tier — they get a Free Plan with $100 credit (up to $200 with onboarding tasks),
and the account **auto-closes after 6 months or whenever credits run out**, whichever is
first. This doesn't meet the "hosted for at least a year" goal, so a second AWS account was
ruled out. Invitrack stays on the existing (pre-July-2025) AWS account, which still has
legacy free-tier terms.

### Why two VMs instead of one
Azure's 12-month free-services bucket isn't limited to one VM SKU — it grants **750 free
hours each, independently, for B1s, B2pts v2 (ARM), and B2ats v2 (AMD)**. Three separate
buckets, not one shared pool. That means two different services can each get their own
full-time free VM simultaneously, at $0, rather than colocating on one host:

- **`cmd/writer` → Standard_B1s** (1 vCPU / 1GB)
- **`cmd/redirector` → Standard_B2ats v2** (2 vCPU / 4GB, AMD-based burstable — also gives
  redirector more headroom, which fits its read-heavy role anyway)

This gives real physical separation for the CQRS-lite split — two independent instances,
independently deployable and independently restartable — not just two containers sharing
one box. See §7 for what this does and doesn't prove about scaling.

### Important correction / timing note
The free 750-hr allotments are part of Azure's **12-month free-services window**, not an
indefinite always-free grant — that's a different, narrower list (Azure Functions, 5GB
Blob, etc.) that doesn't include VM compute. For Azure for Students this window normally
renews on re-verification each year, **but Tejas graduates in 2027 and won't be re-verifying
as a student again** — so this is a one-shot 12-month window, not a renewable one. Decision:
use it fully now rather than delay, since there's no second free year coming after this.
After the 12 months lapse, both VMs revert to standard pay-as-you-go pricing unless
deprovisioned first — revisit before that date (see §9, Rollback).

### Comparison considered

| Provider | Free compute | Duration | Card required | Notes |
|---|---|---|---|---|
| AWS (new account) | credits only | 6 months or until $200 spent | Yes | account auto-closes after |
| **Azure for Students** | **B1s + B2pts v2 + B2ats v2, 750 hrs each** | **12 months, one-shot (no renewal post-graduation)** | **No** | $100 credit covers the small remainder (see §3) |
| GCP | 1x e2-micro VM | Always-free, no expiry | Yes | only one free VM, not two |
| Oracle Cloud | 4 OCPU ARM / 24GB | Always-free, most generous | Yes | weak resume recognition, known for reclaiming idle instances |

Resume angle unchanged: already have AWS experience via Invitrack. This project on Azure
signals cloud-agnostic ability rather than single-vendor lock-in.

---

## 2. Target Architecture

```
        Internet                              Internet
           │                                     │
         443/80                                443/80
           │                                     │
┌──────────┴──────────┐              ┌───────────┴───────────┐
│  Azure VM: B1s        │              │  Azure VM: B2ats v2    │
│  cmd/writer            │              │  cmd/redirector         │
│  (own public IP, own    │              │  (own public IP, own    │
│   TLS via Caddy)          │              │   TLS via Caddy)          │
└──────────┬──────────┘              └───────────┬───────────┘
           │                                     │
           └───────────────┬─────────────────────┘
                            │
              MongoDB Atlas M0 (external,
              always-free, 512MB shared cluster)
```

- **Compute:** two Azure VMs, one per service — no shared host, no internal proxy routing
  between them. Each runs its own binary directly (or in a single Docker container) plus a
  lightweight Caddy instance for TLS termination on that VM alone.
- **Database:** MongoDB Atlas M0, shared by both VMs over the public internet (Atlas
  network access list restricted to the two VM IPs). Always-free regardless of cloud
  provider, removes a whole ops/cost category from either VM.
- **Reverse proxy:** Caddy on each VM individually (not a shared proxy VM) — automatic TLS,
  minimal config, one job per host.
- **IaC:** Terraform provisions both VMs, both NSGs, both public IPs, in one config —
  parameterized per service so the two are clearly parallel, not copy-pasted drift.
- **CI/CD:** GitHub Actions — build both images, push to GHCR, two independent deploy jobs
  (one per VM) so a writer deploy never touches redirector and vice versa — this is the
  actual point of the split, worth reflecting in the pipeline, not just the infra.

---

## 3. Free-Tier Constraints & Cost Estimate

- B1s: 750 hrs/month free. B2ats v2: 750 hrs/month free, separate bucket. Both running
  24/7 (~730 hrs/month) fits comfortably inside each allotment individually.
- **Public IPs are the real cost.** Azure retired free "Basic" SKU public IPs in Sept 2025;
  new IPs must be Standard SKU, roughly $3–4/month each. Two VMs, each independently
  reachable → ~$7–8/month combined.
- Disk: each VM's free allotment easily covers two lightweight Go binaries — not a
  meaningful cost driver here.
- Egress bandwidth: negligible at portfolio-project traffic levels.
- **Estimated total: ~$5–10/month**, driven almost entirely by the two public IPs.
  Comfortably covered by the $100 credit for the full 12-month window, with plenty of
  credit left over — no need to ration it.
- MongoDB Atlas M0: 512MB storage cap, shared cluster — fine for this project's scale, not
  meant for load-testing at production throughput.
- No credit card gymnastics needed (Azure for Students signup doesn't require one).

---

## 4. Networking / Security (draft, not final)

- NSG per VM: allow 80/443 inbound only. No internal service-to-service traffic between
  the VMs (they're independent, that's the point) — the only shared dependency is Atlas.
- Atlas network access list: restrict to the two VMs' public IPs rather than 0.0.0.0/0.
- SSH: restrict to a known IP where possible; plain SSH key auth, password login disabled.
- Secrets (Mongo URI, etc.): `.env` file per VM, excluded from git, not baked into images.
  Azure Key Vault is a possible stretch goal, not required for v1.

---

## 5. Deployment Flow (target state, not yet built)

1. `terraform apply` provisions both VMs, both NSGs, both public IPs in one run.
2. GitHub Actions on push to `main`: run tests → build `writer` and `redirector` images →
   push to GHCR.
3. Two independent deploy jobs, gated on path changes if useful later: each SSHs into its
   own VM, pulls its own image, restarts its own container.
4. Caddy on each VM terminates TLS and forwards to the local service.

---

## 6. Explicitly Out of Scope

- Kubernetes — unnecessary complexity for two free-tier VMs.
- Azure-managed Mongo (Cosmos DB Mongo API, etc.) — costs money; Atlas M0 covers this for free.
- Multi-region / high availability — each service is still a single instance, single point
  of failure per service; accepted tradeoff for a portfolio project.
- A shared front-door/API gateway VM — considered, rejected in favor of each service having
  its own public endpoint, since that's simpler and cheaper than adding a third VM.

---

## 7. What This Does and Doesn't Prove About Scaling

Two separate VMs is real physical separation — genuinely independent deploy targets,
independent restart/failure domains, independent resource profiles (B1s vs B2ats v2 sized
differently on purpose, since redirector is read-heavy and writer isn't). That's a
legitimate, honest demonstration of the CQRS-lite split at the infrastructure level, not
just in code.

What it still doesn't demonstrate: **elastic** scaling — e.g. N redirector replicas behind
a load balancer scaling with traffic, independent of writer's replica count. Each service
here is exactly one instance, always. Getting real elasticity would need a per-service
autoscaling platform (Azure Container Apps, VM Scale Sets) or a load balancer plus a
capacity pool, both beyond what a $100/12-month student credit comfortably sustains.

Interview framing: *"each service runs on its own dedicated instance, sized differently
based on its read/write profile — that's real separation, not colocation. Taking it to
elastic autoscaling is a straightforward infra extension, not a code change, but wasn't
worth the cost for a portfolio-scale project."* That's accurate and defensible.

---

## 8. Open Decisions (resolve before writing Terraform/Caddy config)

- [ ] Domain strategy: one domain with two subdomains (e.g. `api.` / `go.`) vs two
      unrelated free subdomains (nip.io-style) vs just raw IPs to start
- [ ] Whether ADR 0002 (StatsBuffer) ships before or after the first deploy
- [ ] Whether Atlas access list gets locked to the two VM IPs from day one or opened
      temporarily during initial setup/debugging

---

## 9. Rollback / Teardown / Time-Bound Reminder

- `terraform destroy` cleanly removes both VMs, NSGs, and IPs in one command.
- **This free window does not renew** (graduating before the next student re-verification
  cycle) — calendar a check-in ~11 months from provisioning to either deprovision cleanly
  or consciously decide to pay standard pay-as-you-go rates going forward.
- Atlas cluster can be paused or deleted independently of either VM's lifecycle.
# Plan 10: Hosted Services and Network Identities

**Status:** Draft  
**Date:** 2026-08-08  
**Owner:** Implementation team  
**Dependencies:** Plan 03 (module rename) complete; Plan 09 (packaging) partially done  
**Gates:** G8 Services  

## 1. Purpose

Replace all hosted-service endpoints, domains, bindings, cookies, OAuth clients, database deployments, and operational identities used by Patty Code workers with Patty-owned or release-blocked coordinates.

## 2. Scope

- `workers/accounts`: identity routes, cookies, service names
- `workers/crash-report`: ingestion endpoints, D1 bindings, metrics
- `workers/forum`: moderation routes, schema, domain bindings
- `site/` page-level network references
- Go hardcoded endpoint defaults throughout internal packages
- User-agent strings sent to external services

### Exclusions
- Actual production domain provisioning (requires owned domain input)
- Third-party vendor integrations that are not first-party Patty Code

## 3. Task List

### T1: Inventory all external endpoint references
- Scan: `rg 'patty\.io|pattycorp\.' --include '*.go' --include '*.ts' --include '*.astro'`
- Classify each as replaceable, profile-derived, removable, or block-release

### T2: Replace worker config/bindings
- `workers/accounts/wrangler.toml`: replace API route names
- `workers/crash-report/wrangler.toml`: replace database/D1 binding names
- `workers/forum/wrangler.toml`: replace moderation schema refs

### T3: Replace site infrastructure
- Page title metadata sources use Profile.DisplayName
- Download links point to patty package URLs
- Community/contributor pages updated for new product name
- robots.txt, sitemap.xml update

### T4: Update Go-side HTTP client defaults
- User-Agent string: `patty/{version}` → `patty-code/{version}`
- Internal telemetry/reporting endpoint default: empty or placeholder
- Crash reporting: removed from patty base; becomes optional module

### T5: Cookie and session storage naming
- Session cookie name: `patty_session` → `patty_session`
- CSRF token names updated if applicable
- CORS origin policies derived from Profile.WebsiteURL

### T6: Add service-discovery via profile
- All network endpoints resolved through Profile at build time
- Empty strings in dev builds become release-blocked until configured
- No silent fallback to upstream services

## 4. Definition of Done

- [ ] Zero hard-coded patty code.io / pattycorp domain references remain
- [ ] All network endpoints either derived from Profile or blocked
- [ ] Worker configs updated for patty identifiers
- [ ] Site pages reflect new product names
- [ ] User-agent strings updated
- [ ] Gate G8 proof: Patty-owned endpoints pass integration tests; upstream fallback impossible
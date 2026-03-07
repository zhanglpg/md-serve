# Security Dependency Scan Report

**Date:** 2026-03-07
**Project:** md-serve
**Go Version:** 1.24.7

## Dependencies

| Package | Version | Latest | Direct/Indirect |
|---------|---------|--------|-----------------|
| github.com/yuin/goldmark | v1.7.16 | v1.7.16 | Direct |
| github.com/yuin/goldmark-highlighting/v2 | v2.0.0-20230729 | - | Direct |
| github.com/alecthomas/chroma/v2 | v2.2.0 | v2.23.1 | Indirect |
| github.com/dlclark/regexp2 | v1.7.0 | v1.11.5 | Indirect |

## Vulnerability Findings

### No Direct CVEs Found

No CVEs were found targeting any of the four dependencies in the Go Vulnerability Database, NVD, or GitHub Security Advisories.

### Indirect / Go Toolchain Vulnerability

**CVE-2025-58185** — Memory exhaustion in Go's `encoding/asn1` package when parsing maliciously crafted DER payloads. This is a **Go standard library vulnerability**, not a bug in chroma itself. Fedora rebuilt `golang-github-alecthomas-chroma` packages against patched Go toolchains as a precaution. Since md-serve does not use `encoding/asn1` directly, and chroma is used only for syntax highlighting of markdown code blocks, the practical risk is **negligible**.

## Outdated Dependencies

### github.com/alecthomas/chroma/v2 — v2.2.0 → v2.23.1 (significantly outdated)

**Risk: LOW-MEDIUM.** While no CVEs target chroma directly, the current version (v2.2.0, released ~2022) is over 20 major versions behind. Newer versions include:
- Bug fixes and stability improvements
- Additional language lexer support
- Performance improvements

**Recommendation:** Update to v2.23.1. This also updates the indirect dependency `regexp2`.

### github.com/dlclark/regexp2 — v1.7.0 → v1.11.5

**Risk: LOW.** This is an indirect dependency pulled in by chroma. Updating chroma will update this as well. Note that regexp2 uses a backtracking regex engine which is inherently susceptible to ReDoS, but the library provides a `MatchTimeout` field for mitigation. Chroma uses this library internally for syntax highlighting patterns.

## Code-Level Security Observations

### 1. `html.WithUnsafe()` in render/render.go:47

The goldmark renderer is configured with `html.WithUnsafe()`, which allows raw HTML in markdown to pass through unescaped. This is an **intentional design choice** for Obsidian-compatible rendering, but if md-serve is exposed to untrusted markdown input, this could enable **XSS attacks**.

**Risk:** HIGH if serving untrusted content; LOW if serving only trusted/local files.

### 2. Path Traversal Protection in server/server.go:42-49

The server includes path traversal protection via `filepath.Clean` and `strings.HasPrefix` checks. This is correctly implemented.

### 3. No TLS Configuration in main.go

The server uses plain HTTP (`http.ListenAndServe`). This is expected for a local development tool but should not be used for production deployments without a reverse proxy.

## Recommendations

| Priority | Action | Rationale |
|----------|--------|-----------|
| Medium | Update chroma to v2.23.1 | 20+ versions behind; pick up bug fixes |
| Low | Consider removing `html.WithUnsafe()` or adding a flag | Mitigate XSS risk when serving untrusted content |
| Info | Go toolchain is current (1.24.7) | No action needed |
| Info | goldmark is at latest version | No action needed |

## How to Update

```bash
go get github.com/alecthomas/chroma/v2@latest
go mod tidy
```

## Scan Methodology

- Go Vulnerability Database (pkg.go.dev/vuln) — queried for all four dependencies
- GitHub Security Advisories — searched for all packages
- NVD (nvd.nist.gov) — searched for CVEs
- OSV (osv.dev) — attempted query (rate limited)
- govulncheck — attempted but vuln.go.dev was unreachable from this environment
- Manual web search for CVE reports per dependency

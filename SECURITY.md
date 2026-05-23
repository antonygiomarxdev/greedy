# Security Policy

## Supported Versions

| Version | Supported |
|---|---|
| >= 0.15 | ✅ |

## Reporting a Vulnerability

This project handles financial API keys and trading operations. If you discover a security vulnerability, please report it privately.

**Do not** open a public GitHub issue for security vulnerabilities.

Contact: antonygiomarxdev (at) protonmail (dot) com

You should receive a response within 48 hours. If not, follow up.

## Disclosure Policy

- Vulnerability reported → acknowledged within 48h
- Fix developed → patch released within 14 days
- Vulnerability disclosed → after patch is available

## Security Features

- API keys encrypted at rest with NaCl secretbox (XSalsa20-Poly1305)
- Key derivation via Argon2id (memory-hard, 64MB, 3 passes)
- Keys never logged or exposed via MCP list endpoints
- SQLite WAL mode for safe concurrent access
- Constant-time comparisons where applicable
- Idempotency keys prevent duplicate order placement
- Per-bot circuit breakers prevent runaway trading

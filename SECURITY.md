# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in Stackup, please report it responsibly.

**Do NOT open a public GitHub issue for security vulnerabilities.**

Instead, email: deveshpharswan@gmail.com

Include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

## Response Timeline

- **Acknowledgment:** Within 48 hours
- **Assessment:** Within 7 days
- **Fix release:** Within 30 days for critical issues

## Scope

Stackup runs locally on developer machines. Security concerns include:
- Command injection via config values passed to `docker compose exec`
- Path traversal in config file loading
- Environment variable exposure in logs or error messages
- Malicious stackup.yml that could execute arbitrary commands

## Supported Versions

| Version | Supported |
| ------- | --------- |
| latest  | Yes       |

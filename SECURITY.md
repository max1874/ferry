# Security policy

## Supported versions

Ferry has not published a stable release. Security fixes currently target the latest commit on `main`; old commits, debug APKs and private test deployments are not maintained release channels.

## Reporting a vulnerability

Use GitHub's private vulnerability reporting for this repository when it is available. If private reporting is unavailable, open an issue that asks the maintainer for a private contact channel, but do not include exploit details, device tokens, passwords, message contents or uploaded files in the public issue.

Include the affected commit, component, deployment shape, reproduction conditions and impact. Please allow the maintainer time to reproduce and prepare a coordinated fix before public disclosure.

## Deployment boundary

The current Ferry milestone is designed for a trusted private or link-local network:

- transport defaults to HTTP; without `-tls`, LAN users able to observe traffic may read content or credentials;
- the optional shared password controls admission but is not an administrator account;
- admitted devices can manage the shared password and revoke other devices;
- backups contain private messages, files, hashed device tokens and the password verifier.

Do not publish Ferry directly to the Internet. Use a host firewall, a specific private listener address, a high port, and access controls appropriate for the network. Hostile-network hardening and a stable security-support policy are future milestones.

## Local TLS

`-tls` serves HTTPS from a certificate authority Ferry generates and keeps in `<data-dir>/tls`, with key files at mode `0600`. It encrypts LAN traffic and gives the web UI a secure context, which is what the clipboard API requires. Understand these properties before trusting that authority on a device:

- the CA private key never leaves the data directory, but anyone who obtains it can impersonate the names the CA is allowed to certify, on every device that trusts it;
- the CA carries critical name constraints limiting it to `localhost` and loopback, private and link-local IP ranges, plus a serverAuth EKU, so a stolen key cannot be used to impersonate public sites;
- `GET /ferry-ca.crt` is unauthenticated and returns only the public CA certificate, because a device must trust it before it can complete a handshake and join. Compare the SHA-256 fingerprint printed at startup before trusting the file;
- certificates are checked and renewed only at startup, so a server left running past the 398-day leaf lifetime will serve an expired certificate until it is restarted.

`-tls-cert` and `-tls-key` serve a certificate you already own instead; renewal is then yours. See [docs/tls-lan.md](docs/tls-lan.md) for the full boundary and the per-platform trust steps.

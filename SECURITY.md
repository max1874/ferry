# Security policy

> English | [简体中文](SECURITY.zh-Hans.md)

## Supported versions

Ferry has not published a stable release. Security fixes currently target the latest commit on `main`; old commits, debug APKs and private test deployments are not maintained release channels.

## Reporting a vulnerability

Use GitHub's private vulnerability reporting for this repository when it is available. If private reporting is unavailable, open an issue that asks the maintainer for a private contact channel, but do not include exploit details, device tokens, passwords, message contents or uploaded files in the public issue.

Include the affected commit, component, deployment shape, reproduction conditions and impact. Please allow the maintainer time to reproduce and prepare a coordinated fix before public disclosure.

## Deployment boundary

The current Ferry milestone is designed for a trusted private or link-local network:

- transport is HTTP, not HTTPS;
- LAN users able to observe traffic may read content or credentials;
- the optional shared password controls admission but is not an administrator account;
- admitted devices can manage the shared password and revoke other devices;
- backups contain private messages, files, hashed device tokens and the password verifier.

A TLS reverse proxy on a private network or VPN is supported through `-trusted-origin` (`FERRY_TRUSTED_ORIGIN` in Docker). It encrypts only the browser-to-proxy hop; the proxy-to-Ferry hop stays HTTP and should stay on loopback or a private network. The proxy does not make public exposure supported.

Do not publish Ferry directly to the Internet. Use a host firewall, a specific private listener address, a high port, and access controls appropriate for the network. TLS, hostile-network hardening and a stable security-support policy are future milestones.

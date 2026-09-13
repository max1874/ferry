# Contributing to Ferry

Ferry is a public repository with no tagged release. Treat every change as publicly visible, and see `README.md` for what is still unaccepted.

## Before changing code

1. Read `AGENTS.md` and `docs/product-core.md`.
2. Keep the change tied to Ferry's core LAN clipboard/file journey.
3. Do not add TLS, Internet exposure, automatic clipboard capture, background transfer, accounts or store-release behavior without an explicit product decision.
4. Never commit device tokens, passwords, signing files, local Server data or build artifacts.

## Development expectations

- Prefer platform-native code and existing repository mechanisms.
- Keep Server/OpenAPI/Web/iOS/Android contracts aligned when a wire behavior changes.
- Add a regression test for every confirmed defect.
- Report failures visibly; do not hide them behind retries or loading states.
- Preserve `CLAUDE.md` as a symlink to `AGENTS.md`.

Run the gates relevant to your change; the complete command list is in `README.md`. At minimum run `scripts/check-repo.sh`, `git diff --check`, and the tests/build for every touched component. Physical-device claims require physical-device evidence and must remain explicitly unverified when that evidence is unavailable.

## Changes and reviews

Write commit messages and pull requests in English. Use a focused commit message that explains the observable change. In a pull request, include:

- what user problem it addresses;
- which components and contracts changed;
- exact verification commands and results;
- screenshots only when visual behavior materially changed;
- remaining external or device-only validation.

By contributing, you agree that your contribution is licensed under Apache License 2.0.

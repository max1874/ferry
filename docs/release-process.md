# Release process

> English | [简体中文](release-process.zh-Hans.md)

This document owns Ferry's release identity, how a CI candidate becomes a published version, and the acceptance record for 1.0.0. Account secrets never belong here.

## Release identity

- **Decided (Max, 2026-09-14)**: the first version is `1.0.0`, tagged `v1.0.0`. Versions are fixed and published by hand; there is no rolling `edge` or `latest` tag.
- **Decided (Max, 2026-09-14)**: a release publishes the Server + Web image and a deployment bundle. It does not release the native apps; iOS stays on internal TestFlight and Android is built from source.
- Image: `ghcr.io/max1874/ferry:<version>` for `linux/amd64` and `linux/arm64`, one multi-architecture index. The image carries `org.opencontainers.image.revision` with the source commit.
- Bundle: `ferry-<version>-deploy.tar.gz`, a `ferry/` directory holding `compose.yaml`, `.env.example` and `scripts/ferry-data.sh`, with `ferry-<version>-SHA256SUMS.txt`.
- Release notes: `docs/releases/<version>.md` and its Chinese sibling, joined into one bilingual GitHub Release body.
- Authentication: the repository's `GITHUB_TOKEN` only. Every action is pinned to a commit SHA.

## From candidate to release

1. **CI builds the candidate.** Every push to `main` runs the `image` job: it builds both architectures once, checks that the local registry and the OCI archive hold the same index digest, starts each architecture through `compose.yaml`, keeps a device, text, file and password across a restart, and moves a source-built deployment to the image without losing that data. It uploads `ferry-candidate`: the OCI archive, the bundle, `candidate.json` (commit, ref, run, digest, platforms) and `SHA256SUMS`.
2. **The version files already name the version.** `compose.yaml` and `.env.example` default to `ghcr.io/max1874/ferry:<version>`, and `scripts/check-repo.sh` keeps the two equal. Bump both, and add the release notes, in a commit before choosing a candidate.
3. **Acceptance evidence is recorded** in the checklist below, and Max explicitly authorizes the release.
4. **Run the workflow** with the version and the successful CI run on `main`:

   ```bash
   gh workflow run release.yml -f version=1.0.0 -f ci_run_id=<run id>
   ```

   The workflow refuses to continue unless the run is a successful `CI` push run on `main`, its commit is still on `main`, no Release or tag exists for the version, the artifact checksums and digest match the record, and the bundle names the requested image. It pushes the archived image without rebuilding, keeping its digest, and re-reads the registry digest. An existing version under a different digest is a refusal, not an overwrite; the same digest lets a re-run continue.
5. **First release only: make the package public.** GitHub creates a new container package as private. The workflow then fails at the anonymous pull with a link to the package settings. Set the package to Public and re-run the workflow.
6. **Anonymous pull.** With an empty Docker client configuration, the workflow pulls both architectures by tag, checks they resolve to the released digest, and runs the container journey on each.
7. **Draft release.** The workflow creates a draft `v<version>` Release on the candidate commit with the bundle, checksums, bilingual notes, image digest and CI run. Publishing the draft is Max's step and creates the tag.
8. **After publishing**, update the supported-version wording in `SECURITY.md` and `CONTRIBUTING.md` and their Chinese siblings.

## 1.0.0 ship checklist

Frozen on 2026-09-14. Status values: `done` with evidence, `pending` with an owner, or `blocked`.

| # | Item | Status |
| --- | --- | --- |
| 1 | Web: empty-timeline guide and **Connect another device**; Devices connect section with address, copy, local QR code and private-network note; local-only address shows a hint instead of a QR code | implemented; browser acceptance pending (items 9–10) |
| 2 | Startup log names the listener and the browser address separately | implemented; the CI container journey asserts the loopback message |
| 3 | `compose.yaml` pulls the image; `compose.build.yaml` builds the source; `.env.example` holds image and network settings | implemented |
| 4 | CI builds, starts and checks both architectures, source-to-image upgrade and backup/restore self-test against the current source, and keeps the candidate | implemented; pending a green CI run |
| 5 | Release workflow: verified run and artifact, no rebuild, no overwrite, anonymous pull, draft Release | implemented; not yet run |
| 6 | README and this document in English and Chinese; product core records the first-run journey | done |
| 7 | Isolated deployment from an empty directory following the README, restart keeping data, existing source deployment moved to the image, backup and restore | CI covers restart, upgrade and self-test; a README walk-through from the published bundle is pending after the release workflow creates it |
| 8 | Real desktop and phone screenshots and a ~15 second transfer GIF from the final candidate, each file under 3 MiB | pending (needs a real phone) |
| 9 | Real computer and phone browsers: QR join, text send and copy, image preview, file download, without and with a password; record each device and browser | pending (Max's devices) |
| 10 | Local-only hint, IPv4, IPv6, private proxy domain, copy failure, QR failure, offline and reconnect | pending |
| 11 | Package Public and anonymous pull of both architectures | pending the first workflow run |
| 12 | Max authorizes and publishes the draft Release | pending |

Browser acceptance is not native-app acceptance, and CI container runs are not real-device evidence.

## After 1.0.0

Three new users will each deploy Ferry from the README on their own. Record how long each deployment took and where each person got stuck. Ferry collects no telemetry for this.

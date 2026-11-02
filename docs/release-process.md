# Release process

> English | [简体中文](release-process.zh-Hans.md)

This document owns Ferry's release identity, how a CI candidate becomes a published version, and the acceptance record for 1.0.0. Account secrets never belong here.

## Release identity

- **Decided (Max, 2026-09-14)**: the first version is `1.0.0`, tagged `v1.0.0`. Versions are fixed and published by hand; there is no rolling `edge` or `latest` tag.
- **Decided (Max, 2026-11-01)**, extending the original Server + Web release: publish the image, deployment bundle and a signed Android APK. iOS distribution stays separate through TestFlight/App Store; no IPA is attached to GitHub Releases. Android and iOS keep their own app version/build numbers.
- Image: `ghcr.io/max1874/ferry:<version>` for `linux/amd64` and `linux/arm64`, one multi-architecture index. The image carries `org.opencontainers.image.revision` with the source commit.
- Compose download: `ferry-<version>-docker-compose.tar.gz`: a `ferry/` directory holding `compose.yaml`, `.env.example` and `scripts/ferry-data.sh`. It is configuration, not the program; the runnable program is the image. GitHub shows each asset's SHA-256, so there is no separate checksum file.
- **Decided (Max, 2026-09-14)**: the GitHub Release body is English: `docs/releases/<version>.md` without its title and language line, then a link to the Chinese notes and one small line with the image digest, commit and CI run. The notes open by saying which file to download.
- Android download: `FerryDrop-<app-version>-<versionCode>.apk`, the stable release-signed binary exercised on a real device. Before publishing the draft, upload it with `gh release upload` and verify the downloaded bytes and certificate against the accepted package. Do not upload debug or verification-driver builds, app data or signing material. A different development signing certificate requires a user-managed reinstall; do not imply it can update in place.
- Workflow authentication: the repository's `GITHUB_TOKEN` only. Every action is pinned to a commit SHA.
- **Decided (Max, 2026-09-15)**: CI runs only when started by hand (`workflow_dispatch`), never on a push or pull request.

## From candidate to release

1. **CI builds the candidate.** Start CI on `main` by hand with `gh workflow run ci.yml --ref main`. Its `image` job builds both architectures once, checks that the local registry and the OCI archive hold the same index digest, starts each architecture through `compose.yaml`, keeps a device, text, file and password across a restart, and moves a source-built deployment to the image without losing that data. It uploads `ferry-candidate`: the OCI archive, the bundle, `candidate.json` (commit, ref, run, digest, platforms) and `SHA256SUMS`.
2. **The version files already name the version.** `compose.yaml` and `.env.example` default to `ghcr.io/max1874/ferry:<version>`, and `scripts/check-repo.sh` keeps the two equal. Bump both, and add the release notes, in a commit before choosing a candidate.
3. **Acceptance evidence is recorded** in the checklist below, and Max explicitly authorizes the release.
4. **Run the workflow** with the version and the successful CI run on `main`:

   ```bash
   gh workflow run release.yml -f version=1.0.0 -f ci_run_id=<run id>
   ```

   The workflow refuses to continue unless the run is a successful `CI` run on `main` (started by hand, or a push run from before 2026-09-15), its commit is still on `main`, no Release or tag exists for the version, the artifact checksums and digest match the record, and the bundle names the requested image. It pushes the archived image without rebuilding, keeping its digest, and re-reads the registry digest. An existing version under a different digest is a refusal, not an overwrite; the same digest lets a re-run continue.
5. **First release only: make the package public.** GitHub creates a new container package as private. The workflow then fails at the anonymous pull with a link to the package settings. Set the package to Public and re-run the workflow.
6. **Anonymous pull.** With an empty Docker client configuration, the workflow pulls both architectures by tag, checks they resolve to the released digest, and runs the container journey on each.
7. **Draft release.** The workflow creates a draft `v<version>` Release on the candidate commit with the Compose download, English notes linking the Chinese notes, image digest and CI run. Attach the accepted Android APK before publishing. Publishing the draft requires Max's explicit authorization (which may authorize the agent to publish it) and creates the tag.
8. **Point the READMEs at the new version.** After publishing, confirm that the image pulls anonymously and the Compose download URL answers, then change the version in both READMEs: the download commands, the image named in the quick start and the `FERRY_IMAGE` default in Settings. Until then the READMEs keep naming the previous release, never an unpublished download.
9. **After publishing**, update the supported-version wording in `SECURITY.md` and `CONTRIBUTING.md` and their Chinese siblings.

## 1.0.0 ship checklist

Frozen on 2026-09-14. Status values: `done` with evidence, `pending` with an owner, or `blocked`.

| # | Item | Status |
| --- | --- | --- |
| 1 | Web: empty-timeline guide and **Connect another device**; Devices connect section with address, copy, local QR code and private-network note; local-only address shows a hint instead of a QR code | implemented; the join and transfer journey passed in real browsers (item 9); the edge cases in item 10 were skipped |
| 2 | Startup log names the listener and the browser address separately | implemented; the CI container journey asserts the loopback message |
| 3 | `compose.yaml` pulls the image; `compose.build.yaml` builds the source; `.env.example` holds image and network settings | implemented |
| 4 | CI builds, starts and checks both architectures, source-to-image upgrade and backup/restore self-test against the current source, and keeps the candidate | done (2026-09-14): release run 34818862090 accepted the successful CI push run for `84c355b` as its candidate |
| 5 | Release workflow: verified run and artifact, no rebuild, no overwrite, anonymous pull, draft Release | done (2026-09-14): release run 34818862090 pushed the `84c355b` candidate and created the draft Release |
| 6 | README and this document in English and Chinese; product core records the first-run journey | done |
| 7 | Isolated deployment from an empty directory following the README, restart keeping data, existing source deployment moved to the image, backup and restore | done (2026-09-14), with limits: on an arm64 macOS host that already had Docker, with no earlier Ferry container, volume or image, the README quick start ran in an empty directory from the published Compose download. Steps 1–5 took about 13 seconds, pulled the released `linux/arm64` image, and the log named the LAN address to open. A browser on another computer joined directly and showed the Devices QR code; a second, isolated browser session joined, and text, an image and a file crossed between them, the downloaded file matching byte for byte. A restart, then the README backup and restore commands, kept both sessions joined with their messages. The walk-through was run by someone who already knew Ferry, so it does not show where a new user gets stuck (see After 1.0.0). Not covered: a phone (see item 9), the Copy control, the password, and moving a source deployment (CI covers that move) |
| 8 | Real desktop and phone screenshots and a ~15 second transfer GIF from the final candidate, each file under 3 MiB | deferred (Max, 2026-09-14); the README keeps the existing screenshot |
| 9 | Real computer and phone browsers: QR join, text send and copy, image preview, file download, without and with a password; record each device and browser | done (Max, 2026-09-14): on the test server deployment of `84c355b` with host networking, an iPhone camera scan opened Safari and joined; text send and copy, image preview and file download passed without and with a password, with a Mac browser as the other device. Exact iPhone model, iOS version and desktop browser were not recorded. The native apps have no scan entry; scanning always joins through Safari |
| 10 | Local-only hint, IPv4, IPv6, private proxy domain, copy failure, QR failure, offline and reconnect | skipped by Max's decision (2026-09-14); unverified |
| 11 | Package Public and anonymous pull of both architectures | done (2026-09-14): the package was already public. Release run [34818862090](https://github.com/max1874/ferry/actions/runs/34818862090) pulled both architectures without credentials and ran the container journey on each, and an anonymous manifest request from outside CI returned `linux/amd64` and `linux/arm64` under `sha256:3eba51a1d91902543afd91c8f2c3eed85deb450d291a20d6b5599402e41a5bce`, the CI candidate's digest |
| 12 | Max authorizes and publishes the draft Release | done (Max, 2026-09-14T11:27Z): `v1.0.0` points to `84c355b`. After publishing, the asset was renamed `ferry-1.0.0-docker-compose.tar.gz`, the checksum file removed and the body switched to English; the new URL answers anonymously and the old one no longer exists |
| 13 | README quick start switches from the source build to the bundle after the image pulls anonymously and the bundle URL answers; until then the README must not point at unpublished downloads | done (2026-09-14) |

Browser acceptance is not native-app acceptance, and CI container runs are not real-device evidence.

- **Observed (2026-09-14)**: 1.0.0 was built from `84c355b`, before later commits on `main`. Its Web app still carries the icon that `360c300` redrew, and the `compose.yaml` in its Compose download still sets `TZ: Asia/Shanghai`, a maintainer-local default that `main` has since removed; the Alpine image has no time-zone data, so that setting has no reliable effect. Both changes reach users only in the next release. They shipped in 1.0.1.

## 1.0.1 record

- **Observed (2026-09-15)**: `v1.0.1` points to `c7f10d7`, released by run [34922661122](https://github.com/max1874/ferry/actions/runs/34922661122) from CI run [34922312922](https://github.com/max1874/ferry/actions/runs/34922312922) under `sha256:a3dc11c83cc993d5408a2ed528a9f5df9c02bb0766be443d009c14b8ea5e84d5`. Outside CI, an anonymous manifest request returned `linux/amd64` and `linux/arm64`, and the Compose download URL answered anonymously. It was the first release from a manually started CI run.
- **Observed (2026-09-15)**: the first CI run for 1.0.1 failed before the Android build. `setup-android` installs `tools platform-tools` by default, and `sdkmanager` now fails with `Failed to find package 'tools'`; the same runner image had passed the day before, so the SDK repository changed, not Ferry. CI now installs only `platform-tools`, and setup-android v4 still has the old default.
- **Observed (2026-09-15)**: upgrade from 1.0.0 to 1.0.1 followed the 1.0.1 notes on an arm64 macOS host, in an isolated Compose project on loopback. A device, a text, a 200 KB file and an access password were created on 1.0.0; the notes' backup, image change, pull and `up -d` then ran. On 1.0.1 the device token still worked, the message list was identical, the downloaded file matched its SHA-256, a wrong password was refused (401) and the right one joined (201), and the log showed the new container-listener line. The `TZ` line left in the old `compose.yaml` caused no problem. Not covered: restoring that backup, a phone, and amd64.

## 1.0.2 record

- **Observed (2026-10-13)**: `v1.0.2` points to `209e477`, released by run [38364727661](https://github.com/max1874/ferry/actions/runs/38364727661) from CI run [38363293046](https://github.com/max1874/ferry/actions/runs/38363293046) under `sha256:43daceb1059f0e3d287141e6372d0699d8aa6b23e30a1defc788735db3bf471a`. Max authorized the release and its publishing on 2026-10-13. Outside CI, an anonymous manifest request returned that digest and the Compose download URL answered anonymously.
- **Observed (2026-10-13)**: the Server and Web change is the download link (`docs/web-download-links.md`); desktop Chrome saved a downloaded file with its original SHA-256 from an image built at `8371f22`. Max waived the Android in-app browser check.
- **Observed (2026-10-13)**: the maintainer's own deployment moved from 1.0.0 through a local `main` build to the published 1.0.2 image after a `scripts/ferry-data.sh` backup, on the same volume; the container reported revision `209e477` and served the new Web app.

## 1.0.3 record

- **Observed (2026-10-19)**: `v1.0.3` points to `871b592`, released by run [39216949492](https://github.com/max1874/ferry/actions/runs/39216949492) from CI run [39215682042](https://github.com/max1874/ferry/actions/runs/39215682042) under `sha256:881e9273d6ffa3b9d34b8f66b00b39e85fbd0614c574d18d3138a05054def09f`. Max authorized the release and had the agent publish the draft. Outside CI, an anonymous manifest request returned that digest and the Compose download URL answered anonymously.
- **Observed (2026-10-19)**: the Web change is the image loading state and its retry and 60-second timeout (`9a68b8c`). Desktop Chrome against a local Server built at `9a68b8c`, with the image request held in the page: a spinner instead of the broken-image icon in light and dark, a reconnect during the held request loaded the image on a second request, and a request held past 60 seconds became the file card.
- **Observed (2026-10-19)**: the maintainer's own deployment moved from 1.0.2 to 1.0.3 after a `scripts/ferry-data.sh` backup, on the same volume; the container reported revision `871b592` and served the new Web app.

## 1.0.4 record

- **Observed (2026-11-01)**: `v1.0.4` points to `b9709da569837c5215371018456e763b64314ce6`, published by the agent with Max's explicit authorization. [CI 41311622932](https://github.com/max1874/ferry/actions/runs/41311622932) passed all five jobs; [release 41312302524](https://github.com/max1874/ferry/actions/runs/41312302524) published the candidate under `sha256:f831a92a18bc9c37219db4840a608ef7afc04f071f1d44b30054fa79d517f0e6`. An anonymous registry read outside CI matched the digest and both platforms; anonymous downloads of both release assets matched their published bytes. The Compose bundle names `ghcr.io/max1874/ferry:1.0.4`.
- **Observed (2026-11-01)**: the published `linux/arm64` image ran in an isolated OrbStack container. Two device identities received text and a file with original bytes; the recipient recalled both, incremental recalls listed both IDs, repeat recall succeeded, and both the old file URL and an issued ticket returned 404. After restart on the same storage, the recalls stayed absent and a normal later send remained. The file blob was gone and passive logs showed normal startup. The first observation used a dynamic host port that Docker reassigned on restart; reading the new mapping restored access. No product change was needed. CI separately exercised both architectures; this runtime observation was arm64 only.
- **Observed (2026-11-01)**: `FerryDrop-1.0.0-2.apk` is the pristine stable-signed Android package already installed and exercised on a real device, from `36703f0`. No Android source changed between that commit and the release candidate. Draft and public downloads matched SHA-256 `17926d542cf3bc06265ae02547b51a41fdc0744a9cf7309b714ef64434b1ea8a`; its certificate SHA-256 is `e4d615cd77de880201c044034090bb9340a5af11c887b03d006310a00d2786ff`. Real Android send, original-byte file download and cross-device recall were observed before publication. iOS recall is in internal TestFlight as `1.0.1 (1)`; actual-iPhone recall acceptance remains explicitly deferred. No live deployment or device was changed during GitHub publication, and no local test/check suite was run.

## 1.0.5 record

- **Observed (2026-11-02)**: `v1.0.5` points to `6f5315f57212babe8a040c0db41bd83b88994dca`, published by the agent with Max's explicit authorization ("没事直接发吧"). [CI 41371139439](https://github.com/max1874/ferry/actions/runs/41371139439) passed all five jobs; [release 41371544665](https://github.com/max1874/ferry/actions/runs/41371544665) published the candidate. Outside CI, the registry listed `linux/amd64` and `linux/arm64` for `ghcr.io/max1874/ferry:1.0.5`, and the Release page, Compose download and APK download answered anonymously.
- **Observed (2026-11-02)**: `FerryDrop-1.0.1-3.apk` (SHA-256 `ae6f02698546f7a0c1e0a0eaffd2f4adc45565e52c378d37946a8fe754b1154f`, certificate SHA-256 `e4d615cd77de880201c044034090bb9340a5af11c887b03d006310a00d2786ff`, the same as 1.0.0 (2)) was installed over 1.0.0 (2) on the real Android phone, and the APK pulled back from the phone matched the attached download byte for byte. The phone could not reach its server at that time, so the new Android timeline was not seen with real messages; Max chose to release anyway. The Web and iOS Simulator observations are in the release notes.

## After 1.0.0

Three new users will each deploy Ferry from the README on their own. Record how long each deployment took and where each person got stuck. Ferry collects no telemetry for this.

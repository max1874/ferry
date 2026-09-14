# Ferry Product Core

> English | [简体中文](product-core.zh-Hans.md)

## Positioning

- **Decided (Max, 2026-08-29)**: Ferry is a cross-device LAN clipboard and file-sharing project that will ultimately be public and open source, and is self-hosted by its users.
- **Decided (Max, 2026-08-29)**: the product consists of an iOS App, an Android App, a Web app and a Server; the Web app and Server are self-hosted by the user.
- **Decided (Max, 2026-08-29)**: the main interaction is a chat timeline that follows ChatGPT's information hierarchy.
- **Decided (Max, 2026-08-30)**: the iOS App supports iOS 26 at minimum and uses Liquid Glass by default, without maintaining a visual fallback for older systems.
- **Decided (Max, 2026-08-30)**: every device uses the same join flow and device pairing codes are no longer used; the Server deployer chooses between joining directly without a password or configuring one shared access password.
- **Decided (Max, 2026-08-30)**: for the current milestone, devices that join Ferry on the same LAN are treated as trusted. There are no administrator accounts, device ownership or defences against grabbing the first device; the shared password is an optional entry barrier, not an account permission system.
- **Recommended**: the one-line definition is "ferry text and files to your other devices, as easily as messaging yourself".

## Target users and core problem

- **Inferred**: target users own several devices and want their data to stay on machines and a LAN they control.
- **Observed**: traditional file transfer usually splits text, files and history into separate entry points; this repository has not yet implemented anything that verifies the experience.
- **Recommended**: Ferry unifies this content in one chronological message stream that can be scrolled back through.

## Core actions

1. **Decided**: the user deliberately sends a piece of text or a file.
2. **Decided**: other connected devices see it in the same timeline.
3. **Decided**: the user copies the text, or previews and downloads the file.

The first version sticks to deliberate sending. Automatically watching the system clipboard involves accidental transfer, privacy and platform background limits, and is not in the currently confirmed scope.

- **Decided (Max, 2026-09-09)**: Ferry never reads or writes any device's clipboard on its own. Content enters Ferry through the system's own paste and leaves through the copy control on each message. Automatic synchronisation was implemented once and withdrawn in full; the reasons and measured evidence are in `docs/clipboard-sync.md`.

## First-run journey

- **Decided (Max, 2026-09-14)**: a first-time visitor to the GitHub repository must be able to use the README alone to deploy Ferry, join from a phone browser, copy a text and download a file. The default install pulls the published image with a small deployment bundle; cloning the repository or installing Go is not required.
- **Decided (Max, 2026-09-14)**: a phone or computer needs only a browser. Native apps are optional and are described separately from the Web journey, without download links that do not exist.
- **Decided (Max, 2026-09-14)**: an empty timeline tells the user what to send and offers **Connect another device**. The Devices page shows the server address, a copy control and a QR code generated locally in the browser. The code carries only the browser's current origin, never a device token or password, and it is not shown for a local-only address. The empty state follows the real message state, with no first-run flag or invented progress.
- **Decided (Max, 2026-09-14)**: this journey keeps the existing boundaries: private network, deliberate sending, optional shared password, no new HTTP API or database field.

Acceptance for the journey, recorded in `docs/release-process.md`:

1. From an empty directory, following the README, a deployment starts and its log names the address to open.
2. A real phone browser scans the QR code and joins, both without and with a password.
3. Text sent from one device is copied on the other; an image previews and a file downloads.
4. A restart keeps messages, files, device identities and the password; an existing source deployment moves to the image without losing them.

## Product principles

- **Decided**: the user deploys only the Ferry Server; the Web app is served by the same service.
- **Recommended**: the default space serves "my devices", not social relationships.
- **Recommended**: sending, failure, retry and download states must be clear; a chat appearance must not hide transfer errors.
- **Decided (Max, 2026-08-31)**: in the chat timeline, messages sent by the current device appear on the right and messages sent by other devices appear on the left.
- **Recommended**: the UI follows ChatGPT's clear hierarchy, whitespace and composer structure without copying its trademarks, copy or brand assets.
- **Recommended**: privacy comes before a sense of magic; any future automatic synchronisation must be switched on explicitly and explain its boundaries.

## Explicitly not doing now

- Friends, contacts, group management, presence, read receipts and "typing" indicators.
- A third-party cloud-hosted account system.
- Automatically reading and uploading the system clipboard, or automatically writing received messages into it.
- Cross-device clipboard interoperability. Each device handles only its own clipboard.
- Exposure to untrusted networks by default; LAN access must be switched on explicitly, and public-Internet exposure waits for TLS and matching security configuration.
- **Decided (Max, 2026-09-13)**: a deployer's own TLS reverse proxy (a domain inside a private network or VPN) is a supported deployment, and the single origin must be declared explicitly with `-trusted-origin`; this does not change the "no public-Internet exposure" boundary.

## Criteria for feature proposals

First ask: **does it let the user ferry text or files between devices they control more directly and reliably?**

- Reduces sending steps, improves transfer reliability or makes history easier to find again: prioritise.
- Mainly serves social chat, content creation or cloud-drive management: not by default.
- Requires giving up self-hosting, data control or clear authorisation: reject, unless the user redefines the product core.

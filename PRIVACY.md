# Privacy policy

> English | [简体中文](PRIVACY.zh-Hans.md)

Effective date: 2026-09-27

This policy covers the Ferry Server, the Web app it serves, the Ferry iOS App (listed as FerryDrop) and the Ferry Android App.

## The short version

The Ferry project does not run a server, operate an account system or collect any data. Everything you send through Ferry goes only to the Ferry Server that you, or someone you trust, run on your own network.

## What the apps store on your device

- **The Server address** you enter, so the app can reconnect. Android also keeps the device name you enter. iOS does not store a device name: it uses the name of your iPhone as set in iOS Settings, and sends it to your Server when the device joins.
- **A device token** issued by your Ferry Server when the device joins. iOS keeps it in the Keychain; Android encrypts it with a key held in the Android Keystore. Android excludes app data from system backups.
- Files you choose to save leave Ferry and go wherever you save them: on iOS, a location you pick in Files; on Android, a location such as Downloads.

The apps only connect to the Server address you enter. They do not read your clipboard in the background, do not contain analytics, advertising or crash-reporting SDKs, and do not contact any third-party service.

## What the Ferry Server stores

The Server stores what is needed to show the shared timeline:

- messages and uploaded files, exactly as sent, with each file's name, type and size, and for every message the time it was sent and the name, kind (for example iPhone or Android) and ID of the device that sent it;
- for each joined device, its name, its kind, when it joined, and a hash of its token;
- a verifier for the optional shared access password.

This data lives in the Server's data directory on the machine that runs it. The Server's log records startup settings and errors; it does not log message contents. Whoever runs the Server controls this data, including backups and deletion. Revoking a device in the Web app invalidates its token.

## Network security

Ferry is designed for a trusted private network and uses unencrypted HTTP unless the deployer puts it behind their own TLS reverse proxy. Other people on the same network may be able to observe traffic. See [SECURITY.md](SECURITY.md).

## Children

Ferry is a general-purpose tool and is not directed at children.

## Changes and contact

Changes to this policy are published in this file in the Ferry repository with an updated effective date. Questions can be raised through [GitHub issues](https://github.com/max1874/ferry/issues).

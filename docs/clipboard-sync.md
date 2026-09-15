# Clipboard: why there is no automatic synchronisation

> English | [简体中文](clipboard-sync.zh-Hans.md)

## In three sentences

Automatic cross-device clipboard synchronisation was implemented once on 2026-09-08 across Web, iOS and Android, then withdrawn in full together with the LAN TLS introduced for it. It was withdrawn not because it was unfinished, but because all three platforms require the app to be in the foreground and focused to read or write the clipboard, so "automatic" only works while the user is already looking at Ferry, which is exactly when it is least needed. Ferry now never touches any device's clipboard on its own: content enters Ferry through the system's own paste and leaves through the copy control on each message.

This document exists so the next proposal does not have to rerun the same measurements.

## Problem and evidence

1. **Decided (Max, 2026-09-08)**: bring automatic cross-device clipboard synchronisation into scope, with automatic downstream writes, one-tap upstream sending, off by default.

2. **Observed (measured 2026-09-08, Chrome 152)**: on a LAN HTTP page, `navigator.clipboard` does not exist.

   ```
   {"origin":"http://192.168.110.45:8099","isSecureContext":false,
    "hasClipboardObj":"undefined","hasWrite":"undefined","hasClipboardItem":"undefined"}
   ```

   W3C Secure Contexts lists only `https://`, `localhost`, `127.0.0.1` and `file://` as potentially trustworthy origins; private IP ranges are not among them. This is a property of the origin itself, not a browser-version issue. To unlock the Web app, a self-signed local CA and `-tls` were introduced at the time.

3. **Observed (measured 2026-09-08)**: after switching to self-signed HTTPS, `isSecureContext` became `true`, a clipboard write without a user gesture succeeded, and the macOS system clipboard really received the PNG the browser generated. The technical path worked.

4. **Observed (measured 2026-09-08, 4 attempts)**: when the browser window is not the operating system's frontmost window, `navigator.clipboard.writeText` neither resolves nor rejects; Chromium suspends the write until the window regains focus. `document.hasFocus()` returning `true` is not enough to tell. The mitigation at the time was a timeout on every write.

   ```
   {"status":"Ferry could not reach this clipboard: the clipboard did not respond",
    "messages":2,"hasFocus":true}
   ```

5. **Observed**: iOS never allows background access to `UIPasteboard`; from Android 10 only the foreground app or the current input method can read the clipboard, and from Android 12 a read shows the user a notice. All three platforms agree.

6. **Observed (measured 2026-09-09, Chrome 152)**: on a **plain HTTP** LAN page, `document.execCommand('copy')` with a user gesture works, returns `true`, and the content really lands in the macOS system clipboard.

   ```
   page http://192.168.1.20:42817   isSecureContext: false   navigator.clipboard: undefined
   after click navigator.userActivation.isActive: true
   document.execCommand('copy') -> true
   pbpaste -> FERRY-EXECCOMMAND-GESTURE-TEST-20260909
   ```

   The synchronous `execCommand` is not deferred the way the asynchronous Clipboard API is, so the failure mode in evidence 4 does not exist on this path.

## Decisions

- **Decided (Max, 2026-09-09)**: remove all clipboard automation on all three platforms, and remove the LAN TLS introduced for it.

  Reasoning: from evidence 4 and 5, an automatic write on any platform requires Ferry to be in the foreground and frontmost. The user needs something in the clipboard in order to paste it into **another** app, so the flow is necessarily "bring Ferry to the front → switch to the target app → paste". Compared with a copy button on each message — "bring to the front → tap Copy → switch away → paste" — automation saves one tap. The cost is a state machine on each platform (loopback guard, home-screen guard, generation gate, write timeout), an unfixable and invisible failure mode, and a Web-side coupling to "must be HTTPS", because only gesture-free writes need a secure context.

- **Decided (Max, 2026-09-09)**: give up cross-device clipboard interoperability; each device handles only its own clipboard. This explicitly gives up the "copy an image in a Mac browser, paste it straight into a Windows browser" scenario; writing an image into the browser clipboard requires `ClipboardItem`, which exists only in a secure context. After weighing "install a CA certificate once on every device", the user chose to give that scenario up.

- **Decided**: the copy control uses `execCommand('copy')` (evidence 6), not `navigator.clipboard`. That makes the Web app work over plain HTTP, with no secure context, no permission and no write timeout.

- **Decided**: the copy control appears only on text messages. File messages cannot be put into the clipboard through `execCommand`.

## Data and I/O guarantees

| Rule | Mechanism | Counterexample and check |
| --- | --- | --- |
| Ferry does not read the clipboard without a user action | No platform has a code path that reads the clipboard | A repository-wide search finds no `clipboard.read`, no `primaryClip` read and no reader of `UIPasteboard.general.string` |
| Ferry does not write the clipboard without a user action | Writes happen only inside the copy control's click handler | Receiving a new message leaves the clipboard unchanged |
| A write never hangs | `execCommand` returns synchronously, and the click itself proves the window is frontmost | On `false` the button shows `Press Ctrl+C` instead of failing silently |
| No secure context needed | `navigator.clipboard` is not used | The copy button works on a plain HTTP LAN page |

## Known boundaries

- **File messages have no copy button.** `execCommand` can only place plain text. Copying an image would mean returning to `ClipboardItem`, which reintroduces the HTTPS dependency.
- **`execCommand('copy')` is marked deprecated.** Every target browser still supports it. If it is ever removed, Web copying either moves to `navigator.clipboard.writeText` (secure context required) or falls back to the user selecting text and pressing Ctrl/Cmd+C.
- **The Android copy button triggers no system notice on Android 12 and later**, because the notice applies only to reads. Writes need no permission.

## Acceptance

1. On a plain HTTP LAN page, tapping Copy on a text message changes the button to Copied and the system clipboard receives the message's exact text.
2. Receiving a new message leaves the clipboard unchanged.
3. On iOS and Android, after tapping Copy, the same text can be pasted in another app.
4. File messages have no Copy control.

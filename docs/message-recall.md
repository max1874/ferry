# Message recall

> English | [简体中文](message-recall.zh-Hans.md)

## Contract and acceptance — 2026-11-01

Max requested recall of sent content. Decided by Max: any connected device in the trusted space can recall any text, image or file without a time limit, including messages from other devices and legacy messages. Device names and original sender identities do not restrict recall; no new account or ownership system is introduced.

The essential readings are the recipient's timeline and the original file download: a recipient first receives the content, then loses it after recall, and the old download URL and any issued download ticket return 404. After a recipient reconnects or the server restarts, the content remains absent. Normal sending, text copy, image preview and byte-preserving downloads still work.

Finite acceptance:

1. Send text and a file; a second identity receives them and downloads the original bytes.
2. Cancel recall; the content stays. Confirm recall; it disappears on the sender and recipient.
3. Recall an older message after later messages were read, and catch up more than one page of recalls after reconnecting.
4. Another connected device can recall it, including legacy content; a revoked identity cannot recall through an already authenticated request. Retries from any connected device are idempotent.
5. A late poll, send response or image fetch does not restore recalled content. Clear its image cache and dismiss an open preview when the row disappears.
6. Restart with the same storage; content stays absent and interrupted file deletion is completed. Report actual client/runtime coverage separately from builds and CI.

Recall cannot erase copies, completed or in-flight downloads, or backups. Only message IDs and completion sequences remain in the retraction ledger; message bodies and file metadata are deleted. A durable intent precedes unlink. Only after unlink succeeds does one transaction remove the message and assign its retraction sequence. Unlink failure leaves the message and retry control available and publishes no recall; retrying or reopening the store completes cleanup.

## Synchronisation and upgrade boundary

Message sequences and retraction sequences are independent. Clients carry both cursors, apply recalls before appending a page, remember recalled IDs for the session and drain both paginated streams. A message cursor advances only from a fetched page, not a send response. Polling remains the existing foreground mechanism.

Upgrade the server before the clients. New clients can still read/send against older servers, but recall returns an error. Older clients ignore the added retraction fields and can retain already loaded content until reconnect/reload; they cannot provide live recall. The new Web client is served with the server. Native app updates require their separate authorised distribution.

## Scoped interface design

Only timeline rows and recall confirmation change. Their job is to copy/open received content and remove a mistaken send. Keep navigation, composer, typography, colours, spacing and successful send behaviour. Recall is a visible trash icon in the message action row under the content, next to Copy, on every connected device (layout and its 2026-11-01 decision in [DESIGN.md](../DESIGN.md)). While pending, disable that action and label it Recalling…; failures retain the row and use the existing error area so the user can retry. Empty timelines use the existing empty state. Deleting a row does not scroll the user to the bottom.

iOS uses a native confirmation dialog with a destructive Recall and Cancel, following the Mail discard-draft analogy in Apple's [Action sheets](https://developer.apple.com/design/human-interface-guidelines/action-sheets#Best-practices): destructive choices use the destructive role and Cancel dismisses. The dialog states the all-device scope and saved-copy limit. The visible action avoids relying on a hidden context menu ([Context menus](https://developer.apple.com/design/human-interface-guidelines/context-menus#Best-practices), which requires a visible alternative to hidden context actions). Keep a 44-point action target and add no haptic. Android uses a Material confirmation dialog; Web uses the browser confirmation. No new navigation structure is introduced.

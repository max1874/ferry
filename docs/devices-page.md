# Devices main-page navigation

## Scope and decision

- Requested (Max, 2026-09-01): clicking the left-side Devices item must not open a disconnected panel on the far right; visual interactions must be exercised before delivery.
- Done: Timeline and Devices are peer main views controlled by the sidebar. Devices replaces the timeline content, has no modal/backdrop/close affordance, and Timeline returns to the unchanged composer.
- Non-goals: device/password API behavior, revoke confirmation, URL routing, native clients, or a broader sidebar redesign.
- Root cause: `Devices` was styled as navigation but called `dialog.showModal()` on a right-anchored 430 px dialog. The trigger's spatial/semantic promise and result contradicted each other.
- Design: two sidebar buttons own one `activeView`; `selectView` derives main-view visibility and `aria-current`. The Devices page uses the same content column on desktop and the same 56 px rail on mobile.

## Frozen checklist and local journey

1. Initial authenticated view is Timeline with composer visible and `aria-current=page`.
2. Clicking Devices hides Timeline/composer, shows the main Devices page immediately beside the sidebar, and moves active styling/ARIA without a dialog.
3. Device list and password status load; enabling then disabling a local test password produces truthful terminal statuses.
4. Clicking Timeline restores its content/composer and keeps keyboard focus on the selected navigation item.
5. Desktop 1383×997 and mobile 390×844 have no horizontal overflow; rows, revoke control and password card remain inside content bounds.
6. Full Go tests/vet, JS parse, repository policy, diff check, push, retained-volume Mac mini build, and deployed click-through pass.

Local final-tree evidence at base `e13b71a`: desktop sidebar ended x=257 and Devices page began x=257; content was 760 px centered at x=440. Devices click produced `{timelineHidden:true,devicesHidden:false,composerHidden:true,devicesActive:page}` with two rows and `No password is required.` Password enable/disable both reached truthful success text. Timeline click reversed every view predicate. At 390 px, page x=56…390, content/card/rows x=72…374, overflow 0. Recording: `ferry-devices-page-local` (18 frames).

## Author adversarial review

1. Coupled state — `activeView` has one owner and `selectView` derives visibility/active/ARIA; all producers were grepped.
2. Failure paths — device/settings failures stay visible in the Devices status; 401 still returns to access through existing paths.
3. Unchanged consumers — complete HTML, relevant CSS/JS and Web tests reread; Timeline messaging, polling, upload and settings APIs are unchanged.
4. Contract surfaces — no network/schema/storage contract changed; DOM removes dialog semantics and adds main-view navigation semantics intentionally.
5. Original reproduction — deployed dialog measured x=935 while its trigger lived in sidebar x=0…257; the replacement page starts at sidebar edge.
6. Current-HEAD journey — desktop/mobile screenshots and clicks followed the final compact desktop button repair.
7. Mechanism discrimination — both content visibility and `aria-current` reverse when only the clicked navigation button changes.
8. Regression scan — composer accidentally remaining visible was the primary candidate; it is hidden on Devices and restored on Timeline.
9. Scale/edge — mobile rows and card fit; the existing deployed multi-device list is reserved for the final Mac mini replay.
10. Contract attacks — N/A; no external protocol, auth policy, persistence, or universal boundary changed.
11. Predicate producers — every `conversationElement.hidden`, `devicesPage.hidden`, `composerShell.hidden`, active class and `aria-current` writer was enumerated.
12. Reversed findings — the former dialog location is rejected; device loading/error/password questions were rechecked on the main page.
13. Pass limit — author full pass only; no subagent per Max's standing decision, with real browser click-through and machine gates retained as closure evidence.

Status: local candidate.

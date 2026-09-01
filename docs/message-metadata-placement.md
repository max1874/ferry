# Web message metadata placement

## Scope and decision

- Requested (Max, 2026-09-01): “**Mac Web**11:16 AM 这种显示在气泡底下感觉有点不舒服”.
- Done: Web renders device identity and time immediately above the corresponding bubble; the current-device group remains right-aligned and every other group remains left-aligned.
- Non-goals: native clients, metadata content, message identity, API/schema, or a broader timeline redesign.
- Depth: focused. Budget: 2 production files, 3 net production lines, no persistent mechanism; one static-test file and this evidence note.
- Decision: reverse only the existing metadata/body DOM order and margin. This preserves semantics and avoids duplicating metadata or using CSS visual reordering that would disagree with assistive reading order.

## Frozen checklist and evidence

1. Text and file messages append metadata before body — static embedded-resource test.
2. Metadata has a six-pixel gap above the bubble — static CSS contract plus browser rectangles.
3. Current/peer left-right ownership is unchanged — real Chromium with two authenticated identities.
4. Desktop and 390×844 have no horizontal overflow — real Chromium geometry.
5. Go/JS/repository/diff gates pass before push; Mac mini is rebuilt without removing its named data volume.

Local journey: final working tree at base `9def952`; `Mac Web` current/right had metadata bottom 93.20 and body top 99.20, while `Other Mac` peer/left had 182.89/188.89. At 390 px the same gaps were 6 px, right bubble ended x=378, left began x=68, and overflow was 0. Recording: `ferry-message-meta-above-local` (5 frames).

Deployed closure: source `1a2aaab`; Mac mini image `sha256:93826046150c0b8593f2ef6e0206ba7243e6d4d56061898ea9ad62182fec66d0` is running with retained `ferry_ferry-data:/data`, and `/healthz` returned 200. Live desktop and 390×844 Chromium both rendered the user's exact `Mac Web · 11:16 AM` row 6 px above its bubble with overflow 0. Recording: `ferry-message-meta-above-macmini` (4 frames).

## Author adversarial review

1. Coupled state — DOM order and one margin are the only producers; `rg message-head|article.append` enumerates them.
2. Failure paths — no request, async, or error path changed; N/A to this presentation-only correction.
3. Unchanged consumers — complete renderer block, affected CSS, and Web static test reread; file and text share the same wrapper.
4. Contract surfaces — no API/schema/storage change; DOM reading order intentionally changes with visual order.
5. Original reproduction — browser text/rectangles place the metadata before and above both bubbles.
6. Current-HEAD journey — desktop and mobile screenshots came after the final production edit.
7. Mechanism discrimination — `firstElementChild` is `message-head` and its bottom is exactly 6 px before body top.
8. Regression scan — candidate risk was breaking left/right alignment; current ended x=1172/378 and peer began x=468/68.
9. Scale/edge — both text and file use the common append; empty timeline has no affected element; long payload wrapping is unchanged.
10. Contract attacks — N/A; no protocol, policy, schema, auth, or universal external claim changed.
11. Predicate producers — `is-current-device` production is unchanged and still has one strict-boolean renderer producer.
12. Reversed findings — N/A; no earlier finding was overturned.
13. Pass limit — author-focused pass only; no subagent per Max's standing decision, with running-browser and machine gates retained as closure evidence.

# Web icon adaptation

## Scope and decision

- **REQUESTED — Max, 2026-08-30**: “web 也要适配上 icon”。
- **Done**: favicon, Apple touch icon, top-bar brand, connection card, and empty-chat welcome use the repository Ferry icon; no letter placeholder remains.
- **Non-goals**: redesigning the icon, changing layout, API, auth, iOS, or Android.
- **Depth**: focused; two Web production files and two embedded PNG assets, plus one static-resource test.
- **Decision**: reuse the supplied 64 px and 256 px PNG variants. Decorative in-page images use empty alt text because adjacent Ferry/headline text already names their context.

## Frozen checklist and evidence

1. Both PNGs are embedded, served as `image/png`, and decode to exactly 64×64 / 256×256 — `TestHandlerServesIconAndReferencesItFromPage`.
2. HTML contains exactly one top-bar and two welcome `<img>` references and no old letter placeholder — same test plus `rg 'ferry-icon|>F<' internal/webui`.
3. Real browser proves loaded natural dimensions, 28/48 px rendered dimensions, and no letter placeholders — browser-harness current-worktree transcript.
4. Both passwordless timeline and password-required connection card render the icon in dark mode — `/tmp/ferry-icon-qa.png` and `/tmp/ferry-icon-access-qa.png` visual checks.
5. Go tests, JavaScript syntax, `git diff --check`, independent focused review, push, macmini Docker rebuild, and deployed browser check pass.

## Adversarial author review

Verdict after two passes: **ship candidate; no unresolved author-known P0/P1/P2**. Pass 1 found that the test verified only the 64 px file. Independent review then proved both URLs could still serve the same small PNG without failing; pass 2 added exact decoded dimensions and exact `<img>` counts.

1. **Coupled state** — no timer, flag, cache, or application state changed; only embedded bytes, HTML elements, and their CSS selectors. Evidence: `git diff -- internal/webui`.
2. **Failure paths** — either missing/non-PNG/wrong-sized asset or missing visible `<img>` fails the handler test; browser evidence has `complete=true` and nonzero natural width for both sizes.
3. **Unchanged consumers** — `rg 'brand-mark|welcome-mark' internal/webui` finds only the updated HTML/CSS and test; JavaScript has no dependency on element tag or text.
4. **Contract surfaces** — no API, DB, environment, or auth surface changed; only same-origin static paths were added. Evidence: diff stat restricted to `internal/webui`.
5. **Original reproduction** — browser screenshots show the supplied blue Ferry icon where the Web previously showed `F`.
6. **Journey replay** — current worktree ran in the production Go handler at `127.0.0.1:42819`; DOM bodies and screenshots were read after the final production edit.
7. **Mechanism discrimination** — replacing the 256 px bytes with the 64 px file made the test fail `dimensions = 64x64, want 256x256`; restoring it passed. Browser dimensions and empty textContent exclude a letter fallback.
8. **Regression scan** — candidate regressions were layout shift and duplicate accessible names; rendered sizes remain 28/48 px and decorative `alt=""` leaves adjacent labels authoritative.
9. **Scale/edge** — purpose-sized 64/256 px assets are 8/60 KiB; hidden and visible welcome variants both load, while the active one renders at 48 px.
10. **Contract attacks** — N/A: no protocol, schema, credential, or persistence boundary changed.
11. **Predicate producers** — `hidden` is still produced by the existing access/welcome flow; both visible outcomes were forced and observed in the browser.
12. **Reversed findings** — the incomplete asset test re-asked path presence, content identity, dimensions, and placement: both paths now check status/MIME/signature/exact size, while HTML checks exact one-plus-two image counts.
13. **Pass limit** — author evidence is not final green; an independent focused review is required before push.

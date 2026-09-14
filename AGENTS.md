# Ferry — Repository Rules

> English | [简体中文](AGENTS.zh-Hans.md)

This file is Ferry's repository-level collaboration contract and the single authority on project rules. `CLAUDE.md` must always be a symlink to this file; never maintain a second copy of its content. `AGENTS.zh-Hans.md` is a translation: when the two disagree, this English file wins, and both change in the same commit.

## Current stage

- Ferry is a self-hosted LAN clipboard and file-sharing project being prepared for the public, made of a Go Server, an embedded Web app, an iOS 26 App and an Android 8+ App. Product boundaries are defined by `docs/product-core.md`.
- All four components have an MVP. Android has real-device install and launch evidence. The full LAN journey across Web, iOS and Android has never been accepted end to end on real hardware; **Decided (Max, 2026-09-11)**: that is not a precondition for making the repository public. Do not present unit tests or simulator runs as acceptance evidence for that journey.
- Ferry does not currently do TLS, public-Internet deployment, automatic clipboard or background transfer; adding any of these requires an explicit user decision first.
- **Decided (Max, 2026-09-13, issue #1)**: deployers may put Ferry behind their own TLS reverse proxy (such as Caddy) inside a private network; the Server admits that one origin only through an explicit `-trusted-origin`. Ferry itself still does no TLS, never listens on a wildcard address and never reads `X-Forwarded-*`; public-Internet exposure remains unsupported.
- **Decided (Max, 2026-09-11)**: iOS is distributed through TestFlight and is **not released on the App Store**. The App Store Connect record exists under the store name `FerryDrop` (`Ferry` is taken by other developer accounts in en-US and other locales), Bundle ID `com.max1874.ferrydrop`, with `1.0.0 (1)` in internal testing. An App Store release is still out of scope and needs a new explicit user decision. The Team ID, ExportOptions and pipeline needed for distribution live in a private account-side knowledge base, never in this repository.
- LAN TLS and cross-device clipboard synchronisation were implemented once on 2026-09-08 and withdrawn in full on 2026-09-09. Before proposing either again, read `docs/clipboard-sync.md`, which records why they were withdrawn and the measurements already taken; do not rerun the same measurements to reach the same conclusion. Ferry writes the clipboard only when the user taps a copy control, and never reads it.

## Product core before features

- Once the product direction is clear, create `docs/product-core.md` first: target users, core problem, core actions, explicit non-goals and the criteria for judging feature proposals.
- Read `docs/product-core.md` before evaluating or implementing a feature. A feature must serve the core problem; "every common app has it" is not a reason.
- Hard product decisions are confirmed only by the user. Record the basis of each: `Observed` (fact), `Decided` (by the user or an authorised decision), `Recommended` (suggestion); never dress an inference up as a decision.
- When requirements change, update the product core and acceptance criteria together so code, copy and store positioning do not drift apart.

## How to work

- Before starting, read the complete current files, `git status` and the relevant history; do not judge the current state from a diff or old documents alone.
- The repository may have concurrent sessions. Change only the files this task involves; never overwrite or commit someone else's work in progress.
- Work in the current worktree by default. Repository changes the user has decided on are committed and pushed to `origin/main` by default; do not create branches, open PRs, deploy or publish on your own.
- **Decided (Max, 2026-09-13)**: the repository is public and international, so commit messages, PR titles and PR bodies are always written in English.
- **Decided (Max, 2026-09-13)**: every document is provided in English and Simplified Chinese. English lives at the original path and Chinese at the sibling `*.zh-Hans.md`, each linking to the other under its title. A change to either language updates the other in the same commit.
- For a complex feature, freeze a finite, verifiable ship checklist first; when a new problem appears during implementation, decide whether it belongs to the original goal before acting, and never let an investigation replace the goal.
- Fix bugs by finding the root cause and then changing the mechanism; never hide an unknown state behind retries, delays, swallowed errors or UI cover-ups.

## iOS engineering principles

- Prefer Apple native frameworks and SwiftUI by default; the minimum version is decided as iOS 26, and any change to the dependency policy must be recorded explicitly.
- State has a single owner and derived values are computed from source state; avoid several hand-synchronised copies of counts, arrays or caches.
- Asynchronous requests, media playback and navigation tasks must handle stale callbacks, cancellation, failure, retry and release. Use a generation/token gate when needed so old tasks have no right to rewrite new state.
- Resource lifetimes follow interface lifetimes: after leaving, being covered or switching objects, cancel requests and release observers, players and cache usage.
- Error states must be observable; a failure must never be shown forever as loading, a static poster or "looks fine".
- Dates, time zones, permissions, cloud resources and background/foreground transitions are designed against their real boundaries, not defaults that only hold on the development machine.
- Before user data leaves the device or goes to a backend or third-party AI, there must be a clear statement of purpose and authorisation that meets product requirements; minimise collection, persistence and log exposure.
- Never write keys, signing material, personal paths or App Store Connect credentials into source code or this file. Use untracked local configuration or secure storage, and provide examples without secrets.

## Xcode project

- If the project uses Xcode file-system synchronized groups (for example `objectVersion = 77`), new source files join the target through their directory; never hand-edit `project.pbxproj` just to add a file.
- Edit the project file only when build settings, targets, capabilities or resource ownership genuinely need to change; review the diff afterwards to avoid unrelated UUID or ordering noise.
- `xcodebuild clean` is forbidden. Use incremental builds with a separate `-derivedDataPath` outside the repository for each task.
- Do not install new tools or dependencies unless the project needs them and the user has agreed; a new dependency must explain why system frameworks or existing code cannot do the job.

## Simulators and machine load (hard constraints)

- At most 1 booted Simulator and 1 `xcodebuild` at any time; no parallel builds or parallel UI tests.
- After testing, shut down and delete the Simulators this task created; never delete the user's existing devices or data.
- Do not trigger `mediaanalysisd`: do not bulk-import media into the photo library, and do not import large images or video. When media acceptance is genuinely needed, reuse a one-off seed kept to the smallest count and size that proves the behaviour.
- Keep screenshots only for key acceptance points, all written to a temporary directory outside the repository or to a git-ignored artifacts directory inside it whose name ends in `.noindex`.
- Do not start heavy long-running processes unrelated to the task.

## Verification standard

- "Verified" must come with reproducible evidence: command output, test results, screenshots or a real-device install receipt; "it should work" does not count.
- Verify the user behaviour itself, not a proxy signal. For example, prove playback by time advancing, not by two screenshots differing; for asynchronous completion assert the final state, not merely that a button disappeared.
- Prefer unit/integration tests for logic, state transitions, races and regressions; use the fewest simulator or real-device checks for visual layout and interaction. Do not disable tests in Ferry by default just because an earlier project removed them.
- A skipped test is never a success; failure, cancellation, empty values, stale callbacks, repeated actions and teardown are baseline cases for asynchronous features.
- Before delivery, run at least the build/tests proportionate to the risk and `git diff --check`, and do one adversarial self-review round for non-trivial changes.

## Documentation discipline

- `AGENTS.md` holds long-term constraints; `docs/product-core.md` holds product boundaries; complex design documents hold the problem, evidence, candidates, decision, counterexamples and acceptance.
- Documents record only trade-offs, external contracts and pitfalls that cannot be read from the code. Do not copy file lists, view responsibilities or API lists; they go stale, and the current code is the authority.
- Historical design documents may describe targets, tests or architecture that have since been deleted. Cross-check them against the current project and source before acting on them.

## Releases and external actions

- TestFlight, App Store submission, production deployment, data migration and external messages all require explicit user authorisation; a successful build is not permission to release.
- Once release identity, language coverage, privacy answers and release policy are settled, write them into a dedicated release document, never account secrets into the repository.
- Keep reversible pre-checks separate from irreversible submissions. Before submission, check the locales actually supported, version state, build number, privacy declarations and store assets; do not copy and keep locales that have no real translation.
- When there is a companion backend, gather the release order, failure semantics and acceptance of client and compatible backend into one script; the script fails closed, and completion is claimed only when every release surface has been verified for real.

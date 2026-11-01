# Ferry design record

> English | [简体中文](DESIGN.zh-Hans.md)

Max's design verdicts, newest first. Each one states what was shown, what he chose or rejected, his words and the date. A rejected element does not come back without a new verdict.

## Message action row — 2026-11-01

- **Shown**: every client put Copy and Recall as small text links in the line above the message (`iPhone · 11:24 PM · Copy · Recall` on Web, `iPhone Copy Recall` on iOS and Android). The Web link was 35×13 px.
- **Verdict (Max)**: “感觉现在的 copy 按钮做的不够好” (“the current copy button isn't good enough”), then, pointing at ChatGPT Web's icon row under an answer: “学一下 chatgpt web 的 ui 吧” (“learn from ChatGPT Web's UI”). An explicit direction.
- **Rejected (Max)**: moving Recall into a “…” overflow menu to avoid mistaps: “recall 有二次确认，怕什么误触” (“Recall already asks for confirmation; why worry about mistaps”).
- **Now**: the line above the message holds only identity and time. Under the content, a row of quiet icon buttons, left-aligned with the content (right-aligned under the current device's Web bubbles): Copy (copy icon, which turns into a checkmark for 1.6 s after copying) and Recall (trash icon, followed by the existing confirmation). Icons use the secondary text colour, gain contrast on hover or focus on Web, and carry their names as accessible labels and tooltips. Targets: 32 px with a mouse and 40 px with touch on Web, 44 pt on iOS, 48 dp on Android; the first glyph lines up with the content's edge.

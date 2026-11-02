# Ferry design record

> English | [简体中文](DESIGN.zh-Hans.md)

Max's design verdicts, newest first. Each one states what was shown, what he chose or rejected, his words and the date. A rejected element does not come back without a new verdict.

## Phone timeline like ChatGPT — 2026-11-02

- **Shown**: FerryDrop on his Android phone after the icon row below: every message had a black square avatar, a bold sender name with the time on the far right, blue “FILE” tiles, and 48dp icon buttons on a 48dp pitch.
- **Verdict (Max)**: “怎么手机上这么丑啊” (“why does it look so ugly on the phone”), with a ChatGPT iPhone screenshot. Offered three scopes (full ChatGPT structure, match the Web bubbles, icon row only); he picked **full ChatGPT structure**, an explicit choice.
- **Now (iOS and Android only; Web keeps its bubbles)**: no avatars. Each message has one small secondary line, “device · time”. Other devices' text is plain full-width body text on the leading side; this device's text is a light grey bubble on the trailing side, at most 85% wide, with its meta line and actions on that side too. Images keep their rounded shape; file cards are neutral surfaces with a line file icon, no coloured tile. Action icons are 18 pt/dp on a 36 pt/dp pitch, the outer glyph on the content's edge, as in ChatGPT's measured row (about 18 pt glyphs on a 32 pt pitch). That is below Apple's 44 pt and Material's 48 dp touch-target guidance, a trade accepted for this row.

## Message action row — 2026-11-01

- **Shown**: every client put Copy and Recall as small text links in the line above the message (`iPhone · 11:24 PM · Copy · Recall` on Web, `iPhone Copy Recall` on iOS and Android). The Web link was 35×13 px.
- **Verdict (Max)**: “感觉现在的 copy 按钮做的不够好” (“the current copy button isn't good enough”), then, pointing at ChatGPT Web's icon row under an answer: “学一下 chatgpt web 的 ui 吧” (“learn from ChatGPT Web's UI”). An explicit direction.
- **Rejected (Max)**: moving Recall into a “…” overflow menu to avoid mistaps: “recall 有二次确认，怕什么误触” (“Recall already asks for confirmation; why worry about mistaps”).
- **Now**: the line above the message holds only identity and time. Under the content, a row of quiet icon buttons, left-aligned with the content (right-aligned under the current device's Web bubbles): Copy (copy icon, which turns into a checkmark for 1.6 s after copying) and Recall (trash icon, followed by the existing confirmation). Icons use the secondary text colour, gain contrast on hover or focus on Web, and carry their names as accessible labels and tooltips. Targets: 32 px with a mouse and 40 px with touch on Web; the first glyph lines up with the content's edge. (The phone targets were later changed by the entry above.)

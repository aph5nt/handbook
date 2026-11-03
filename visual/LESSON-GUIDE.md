# How to write a visual lesson

The reader has dyslexia and a strong visual memory. They learn from a
picture that moves, a concrete story, and a reason. They do not learn from
lists of facts. Every lesson is one standalone HTML file, opened from disk by
double-click. No build step, no server.

Copy `2pc-vs-saga.html` as the starting point. Keep its CSS tokens, fonts,
controls, card and table styles. Replace the scene and the steps.

## The six parts, in this order

1. **One concrete story.** A real thing happening: an order, a cache miss,
   a block being indexed, a user logging in. Named actors. Never abstract
   "Service A".
2. **Animated step-through.** An inline SVG scene. Buttons: Back, Next,
   Play, Reset, and a "Step i of n" counter. Each step changes the picture
   (a colour, a message arrow, a counter, a pointer moving over an array)
   and the narration card under it. Five to nine steps. Colours carry
   meaning and the legend says what each colour means.
3. **The failure that matters.** The last step or a second tab shows the
   failure an interviewer probes. The picture must show it, not just say it.
4. **Where you meet it.** Two short cards: real systems where this shows up
   (a database driver, Kafka, Redis, Kubernetes, a Go indexer writing to
   StarRocks, a Claude Code agent) and the practical choice you make there.
5. **Side by side.** A compare table when the concept has a rival.
6. **One interview sentence.** The single line to say out loud, in a row of
   the table or a highlighted card at the end.

## Code panel and mechanism

Every lesson has a code panel (pseudo code and Go, Python or SQL) whose lines
light up per step, and every narration step explains how the mechanism
decides. Rules and the reference lesson: `CODE-GUIDE.md`.

## Rules

- Standalone: `<!doctype html>`, `<html>`, `<head>` with charset, viewport
  and `<title>`, inline CSS and JS. The only external request allowed is
  the Google Fonts stylesheet already in the prototype. No CDN scripts.
  No Mermaid. Draw the scene in SVG by hand.
- Works at 400px width. The SVG is `width:100%` inside `.stage` with
  `overflow-x:auto`.
- Light and dark theme from the token block already in the prototype.
- `prefers-reduced-motion`: show the step at once, no transitions.
- Narration card: title (what happened), body (two or three plain
  sentences), and the `why` line (what it means, the consequence). Short
  sentences. Common words. Every abbreviation expanded once.
- Numbers only when they are true and general (an L1 cache hit is about a
  nanosecond). Never invent measurements.
- Tabs (as in the prototype) when two protocols or two algorithms should be
  compared on the same scene.
- 150 to 350 lines per file. If the concept needs more, split it into two
  lessons.
- File name: `NN-slug.html` where NN is the chapter number from
  `../README.md`, e.g. `05-cache-stampede.html`.
- When done, do not edit `index.html`. Report the file names and a one-line
  description of each; the lead adds them to the index.

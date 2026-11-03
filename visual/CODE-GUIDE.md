# Code in every lesson and every section

aphex5 asked (2026-11-02): every example shows **pseudo code** and a **real
implementation in Go or Python**, and every animation explains **how** the
mechanism works, not only what happens.

Reference: `visual/16-batch-vs-stream-watermarks.html` and
`code/16-watermarks/`. Copy its code panel.

## 1. The narration explains the mechanism

Each step's body names the check or rule that runs, with the real values.

- Weak: "A late swap arrives. The rule emits a correction."
- Strong: "Its window is 12:00. The check asks: has the watermark reached
  12:00's end plus 60 seconds, that is 12:02:00? The watermark is 12:01:30.
  No. So the state still exists and the swap is counted."

A reader must be able to rebuild the algorithm from the steps alone. If a
step contradicts an earlier one (state "dropped" and later "corrected"), the
model is wrong. Fix the model first.

## 2. The code panel in the lesson

Under the narration card, before the legend:

- A card titled "The code that runs this step".
- Two buttons: `Pseudo code` and the real language (`Go`, `Python` or `SQL`).
- A `<pre>` with numbered lines. Each step object carries `pc:[line numbers]`.
  Those pseudo-code lines light up. A `map` turns pseudo lines into real
  lines, so the real view lights up the matching lines.
- 10 to 20 pseudo-code lines. 20 to 50 real lines. Comments in plain words.
- The failure tab lights the line that is missing or switched off, and the
  fix step lights the line that repairs it.
- Each tab may have its own code (the batch tab of the reference shows SQL).

## 3. The runnable program

- Put it in `handbook/code/<NN-slug>/`: `main.go` plus `go.mod`, or `main.py`.
- `main` replays the exact story of the lesson and prints each step, so the
  numbers in the animation are proven by a run.
- Check it: `gofmt -l .`, `go vet .` and `go run .`, or `python3 main.py`.
  The printed numbers must match the lesson's numbers.
- Standard library only. No network.

## 4. Which language

| Chapters | Real code |
|---|---|
| 01 to 06, 11 to 15, 18 | Go |
| 07, 08 | SQL, with Go where a caller matters (a pool, a retry, a batch insert) |
| 09, 10 | Python |
| 16 | Go or SQL, whichever the real system uses |
| 17 | Python |

## 5. The chapter section

Each concept section in the Markdown chapter gets a part named
`### In code`, after "How it works, step by step":

1. A `text` code block with the same pseudo code as the lesson.
2. A `go`, `python` or `sql` code block with the core of the real code.
3. One line: `Run it: handbook/code/<NN-slug>/` with the command.

## 6. Checks

- `node --check` on the extracted script.
- Headless Chrome: every step of every tab, both code views, no error.
- A screenshot of one step with the real code shown: the lit lines match the
  step.
- The program runs and its output matches the lesson.

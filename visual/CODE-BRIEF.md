# Round 2 brief: code and mechanism in every lesson and section

aphex5's words (2026-11-02): "no idea how we handle here the late event, i see
a nice animation but the explanation is missing; we also need pseudo code in
all examples ... pseudo code + golang or python implementation as example".

Read `CODE-GUIDE.md` first, then open the reference lesson
`16-batch-vs-stream-watermarks.html` and its program `../code/16-watermarks/`.

For **every** lesson of your chapter (`visual/NN-*.html`) and every concept
section of your chapter:

1. **Mechanism.** Re-read each step. Rewrite any step that says what happened
   but not how it was decided: name the check, the rule and the real values.
   Look for contradictions between steps; fix the model first.
2. **Code panel.** Add the panel exactly as the reference does it: pseudo code
   and real code buttons, numbered lines, `pc:[...]` per step, a `map` from
   pseudo lines to real lines. Failure tabs light the missing line, fix steps
   light the repair.
3. **Program.** `handbook/code/<NN-slug>/` with `main.go` and `go.mod`, or
   `main.py`. It replays the lesson's story and prints the numbers. Standard
   library only. Run `gofmt -l .`, `go vet .`, `go run .` or `python3 main.py`.
   A concept with no runnable core (a team practice, a diagram style) gets
   pseudo code only, and says so in the panel.
4. **Chapter section.** Add `### In code` after "How it works, step by step":
   the pseudo code (a `text` block), the core real code, and
   `Run it: handbook/code/<NN-slug>/` with the command.
5. **Checks.** `node --check`. Headless Chrome walks every step of every tab
   in both code views with 0 errors. One screenshot per lesson with the real
   code shown: the lit lines match the step. mmdc over the chapter, no `;` in
   Mermaid. Every program runs and matches the lesson.

Rules:
- Use your own scratch folder `<scratchpad>/<your-name>/`. Never write in
  another agent's folder.
- Edit only your chapter, its `visual/NN-*.html` files and `code/NN-*/`.
  Never edit `16-batch-vs-stream-watermarks.html`, `index.html`, `README.md`.
- Use helper agents if you need them; you own their result.
- Do not commit. Report: lessons done, programs written and run, anything
  that is pseudo code only and why.

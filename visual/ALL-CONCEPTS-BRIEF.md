# Brief: one lesson for every concept in one chapter

You own one chapter, named in your task. Paths are under /home/aph5nt/work/resume/handbook/.

1. Read visual/LESSON-GUIDE.md, then two reference lessons: visual/03-2pc-vs-saga.html and one lesson that already exists for your chapter. Copy their structure exactly: tokens, fonts, controls (Back, Next, Play, Reset, step counter), narration card, legend, compare table, reduced motion.
2. Read your chapter. Every `## ` section between "The map in words" and "Small words" is a concept. Every concept that has no `▶ Watch it move` line needs a lesson. Build all of them. Do not skip any.
3. Each lesson uses the same story, actors, steps and failure as its chapter section, so what the reader watches matches what they read. Five to nine steps per tab. A failure tab or failure step in each. Name the file `NN-slug.html`, NN being your chapter number.
4. After each lesson, add one line at the end of its section in the chapter: `▶ Watch it move: [Title](visual/<file>.html)`. Change nothing else in the chapter.
5. Check each lesson: no `<script src`, ends with `</html>`, the extracted script passes `node --check`, and in headless Chrome (/usr/bin/google-chrome) every step of every tab runs with no JavaScript error. Look at one screenshot per lesson and fix overlapping labels.
6. Do not edit index.html, LESSON-PLAN.md, README.md or any other chapter. Do not commit. Report the files you made, one line each.

## Added: entire content, every term (aphex5, 2026-11-02)

The examples aphex5 gave were samples, not the full scope. Every term the chapter names gets the full treatment.

7. When your lessons for the existing sections are done, go through the "Small words" table. Every term an interviewer could ask "explain X" about (for example anti-corruption layer, invariant, specification, shared kernel, outbox, projection) becomes its own full `## ` concept section in the chapter, with all six parts from TEMPLATE.md (picture, story, steps, what goes wrong, where you use it, one sentence), placed next to the concept it belongs to. Then build its lesson and add its Watch it move line.
8. A term that is only a plain word (such as "Domain") may stay in the table, but its line becomes three or four plain sentences with one concrete example.
9. Then check the chapter covers every term in its own "Core terms" line of handbook/README.md and every term a senior interviewer would name for this topic. Add any that are missing, the same way.
10. Mermaid: no `;` inside a block. Render-check the chapter with `~/.npm/_npx/0a989c03107e8b3e/node_modules/.bin/mmdc -p <puppeteer json with executablePath /usr/bin/google-chrome and args --no-sandbox> -i <chapter> -o <scratch>/out.md` and fix any failure.
11. Practise covers the whole chapter: at least one question per concept group, more where interviewers probe. The 10-question cap in TEMPLATE.md no longer applies. Update the summary map at the top of the chapter so it shows every concept section.

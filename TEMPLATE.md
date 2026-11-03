# Handbook chapter template: learn by seeing, not by memorising

The reader has dyslexia and a strong photographic memory. Text facts do not
stick. Pictures, stories, motion and practical use do. Every chapter is built
so the reader **understands** each idea and can rebuild it from a picture in
their head. Answers in an interview then come from understanding, not recall.

## The shape of every concept

Each important concept in a chapter is one section with these six parts, in
this order. Never a bullet list of definitions.

1. **A picture first.** A Mermaid diagram (GitHub renders it) or an
   embedded SVG in `img/`. The picture alone should tell most of the story.
   Pick the form that matches the idea:
   - flow or protocol: `sequenceDiagram`
   - lifecycle: `stateDiagram-v2`
   - structure, who talks to whom: `flowchart`
   - data shape: `erDiagram`
   - before and after: two subgraphs side by side in one `flowchart`
2. **A story.** Three to six short sentences. A real situation with named
   actors: a customer orders a phone, an indexer reads block 9,506, an agent
   asks a tool for a balance. Never "Service A calls Service B".
3. **How it works, step by step.** A numbered list that walks the picture.
   One action per step. Each step points at a box or arrow in the picture.
3b. **In code.** A `### In code` part: the pseudo code, then the real code
   in Go, Python or SQL, then `Run it: handbook/code/<NN-slug>/`. Rules in
   `visual/CODE-GUIDE.md`.
4. **What goes wrong.** The one failure an interviewer probes, drawn if it
   can be drawn (a red node, a second diagram), then the fix.
5. **Where you use it.** Two or three real systems or decisions, with the
   practical choice you make there. This is what turns theory into
   experience.
6. **Say it in one sentence.** A quote block with the single line to say
   out loud. This is the only thing to remember word for word.

Under the section, if a visual lesson exists, link it:
`▶ Watch it move: [Two-phase commit versus saga](visual/03-2pc-vs-saga.html)`

## The chapter around the concepts

```
# N. Topic

> One line: what this is and why it matters.

<one big picture of the whole topic: a flowchart or mindmap that shows how
 the concepts in this chapter connect, so the reader sees the map first>

## The map in words      three to five sentences walking the big picture

## <Concept 1>           the six parts above
## <Concept 2>
...                      the 6 to 12 concepts that matter most

## Small words           a table: term | one plain line | which concept above it belongs to
                         for every minor term the interviewer may say

## Practise              6 to 10 interview questions. Each answer is a short
                         paragraph that points back to a picture above
                         ("Draw the 2PC sequence and point at the crash"),
                         never three memorised variants.

## Read next
```

## Writing rules

- Short sentences. One idea per sentence. Common words.
- Define every abbreviation the first time it appears.
- Bold only the one key word in a paragraph, never whole sentences.
- No walls of text: no paragraph longer than four lines.
- Every concept section has its own picture. A chapter has at least as many
  diagrams as concepts.
- Numbers only when true and general (orders of magnitude). Never invent
  measurements.
- Mermaid labels never contain a semicolon. Quote labels with brackets or
  parentheses: `A["Order (root)"]`.
- Keep examples from the author's real world where they fit: a Go EVM
  indexer writing to StarRocks (a columnar warehouse), a graph sync into
  DozerDB (a Neo4j-compatible graph database), an actor-model IoT backend on
  Akka.NET, a Claude Code agent harness with skills, hooks and evals, a
  local MCP proxy. Use public systems (Netflix, Kafka, Postgres) otherwise.
- 400 to 800 lines per chapter. Depth through pictures, not through more
  bullets.

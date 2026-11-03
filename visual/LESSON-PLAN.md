# Visual lesson plan

Done lessons are marked ✅. Every other line is still to build. Each line is
the brief: the story, what the steps show, and the failure.

## 1. Domain-Driven Design
- ✅ `01-bounded-contexts.html`
- ✅ `01-aggregate-root.html`

## 2. Architecture styles
- ✅ `02-hexagonal.html`: a core domain in the middle, ports as sockets, adapters plugged in (REST, CLI, test, Postgres, in-memory). Steps swap the database adapter and run the same core under a test adapter. Failure: business logic leaking into a controller.
- ✅ `02-strangler-fig.html`: a legacy monolith behind a routing facade. One route at a time moves to a new service, the legacy shrinks, the last route moves, the legacy is switched off. Second tab: the big-bang rewrite that never ships.

## 3. Distributed systems
- ✅ `03-2pc-vs-saga.html`
- ✅ `03-raft-election.html`
- ✅ `03-cap-and-quorum.html`: three replicas, N=3. Tabs W=1 R=1 (fast, a stale read shown), W=2 R=2 (the overlap guarantees the latest write), and a partition where the minority side must refuse (consistency) or answer stale (availability).
- ✅ `03-outbox-and-idempotency.html`: a service writes an order row and an outbox row in one transaction, a relay publishes, the consumer applies with an idempotency key. Failure tab: without the outbox, the write succeeds and the publish fails, or a duplicate delivery charges twice. Show the key table rejecting the duplicate.

## 4. Messaging
- ✅ `04-kafka-partitions.html`: a topic with three partitions, producers hash a key to a partition, a consumer group of two then three consumers, offsets per partition. Ordering holds per key. A consumer dies and partitions rebalance. Lag grows and shrinks. Failure: a hot key sends everything to one partition.
- ✅ `04-delivery-guarantees.html`: tabs at-most-once, at-least-once, effectively-once. Same message, same crash point between processing and the acknowledgement. Show what is lost, what is doubled, and how an idempotent consumer makes at-least-once safe.
- ✅ `04-dead-letter-queue.html`: a poison message, retries with exponential backoff shown as growing gaps on a timeline, then the dead-letter queue, an operator fixes and replays. Failure: infinite retry blocking the whole queue (head-of-line blocking).

## 5. Caching
- ✅ `05-cache-levels.html`
- ✅ `05-cache-aside-vs-write-through.html`: tabs cache-aside, write-through, write-behind. Same reads and writes against app, cache and database. Show the stale window in cache-aside after a write, and the lost-write risk in write-behind on a crash.
- ✅ `05-cache-aside.html`: Cache-aside: the miss, the load, the set with a TTL, and why the delete goes after the commit.
- ✅ `05-cache-stampede.html`: a popular key expires, 50 requests drawn as dots all miss and hit the database, which falls over. Second tab: request coalescing (one goes, the rest wait) and jittered expiry. Show the request count at the database in both.
- ✅ `05-lru-eviction.html`: a cache of 4 slots plus a recency order. Step through gets and puts, the least recently used slot is evicted. Second tab LFU (least frequently used) on the same sequence, with a different victim.

## 6. Actor systems
- ✅ `06-actor-mailbox-and-supervision.html`: one actor per IoT device, each with a mailbox, one message at a time. A message throws, the supervisor restarts the actor, the mailbox is kept, processing continues. Tabs for restart, resume, stop, escalate on the same failure. Failure: mailbox overflow when a slow actor is flooded.
- ✅ `06-cluster-sharding.html`: 100 device actors over 3 nodes by shard id, a message routed by id to the right node, a node dies, its shards move and the actors rebuild from stored events. Failure: split brain, two nodes own one shard, and the split brain resolver.

## 7. SQL fundamentals
- ✅ `07-join-types.html`
- ✅ `07-join-algorithms.html`: two small tables. Tabs nested loop (inner scan per outer row, a comparison counter), hash join (build on the small side, probe with the big side), sort-merge (both sorted, two pointers). The counter shows n×m against n+m.
- ✅ `07-isolation-levels.html`: two transactions on a timeline over one row. Tabs read uncommitted, read committed, repeatable read, serializable. Each shows the anomaly it allows: dirty read, non-repeatable read, phantom, write skew. The anomaly flashes red.

## 8. SQL optimisation
- ✅ `08-seek-vs-scan.html`: a table of a million rows drawn as a long strip, a B-tree above it. Tab scan: the pointer sweeps the strip. Tab seek: root to leaf in 3 hops, then a short range. A pages-read counter. Failure: `WHERE lower(email) = ...` turns the seek back into a scan.
- ✅ `08-partition-pruning.html`: a table partitioned by day, 30 boxes. `WHERE day BETWEEN` lights 3. Second tab: a cast or function on the day column lights all 30. Third: a columnar store reads 2 of 12 columns (projection pushdown), scan bytes shrink.
- ✅ `08-n-plus-one.html`: an app loads 1 order then 1 query per line, arrows multiply as n grows to 20. Second tab: one join or one `IN (...)`, two arrows. Third tab: a daemon writing one `INSERT` per row against one batched `INSERT` per page, with a statement counter. Tie to the Go indexer and StarRocks story in chapter 8.

## 9. Agentic design
- ✅ `09-react-loop.html`
- ✅ `09-workflow-vs-agent.html`: tabs. A fixed workflow (prompt chain with a gate) and an agent that picks its path. Same task. Cost and predictability bars. The rule: workflow when the path is known, agent when it is not.
- ✅ `09-rag-pipeline.html`: documents chunked, embedded as points on a plane, a query embedded, nearest neighbours lit, a re-ranker reorders, the top 3 go into the prompt, the answer cites them. Failure tab: chunks too big or too small, and a poisoned chunk carrying an instruction (prompt injection through retrieval).
- ✅ `09-supervisor-workers.html`: a supervisor splits a task into three briefs, three workers in isolated worktrees run in parallel, each reports DONE or BLOCKED, the supervisor merges. Failure: a worker claims success with no evidence, and the verifier step that catches it.

## 10. LLM systems
- ✅ `10-tokens-and-kv-cache.html`: a prompt becomes tokens, prefill fills the KV cache in one pass, decode adds one token at a time. Time to first token and tokens per second as two bars. Second tab: prompt caching reuses the shared prefix and prefill becomes almost free.
- ✅ `10-non-determinism-testing.html`: one prompt run 10 times, answers scatter, some wrong. One run proves nothing. Then the eval: a golden set, a pass rate with a threshold, and with-versus-without-skill as two distributions. Failure: one good run declared a win.

## 11. Famous architectures
- ✅ `11-netflix-request.html`: a play request: device, Open Connect CDN for the video bytes, the API gateway (Zuul), service discovery (Eureka), a service calling three others, one slow, the circuit breaker opens and a fallback answers. Second tab: Chaos Monkey kills an instance, traffic reroutes.
- ✅ `11-twitter-timeline-fanout.html`: tabs fan-out on write (a tweet pushed into every follower's timeline) and fan-out on read. The celebrity with millions of followers, and the hybrid.
- ✅ `11-uber-doma-and-cells.html`: hundreds of services grouped into domains behind gateways (DOMA), then cells: the same stack copied per cell with a thin router. One cell fails, only its users turn red.

## 12. Scalability and reliability
- ✅ `12-circuit-breaker.html`: client, breaker, flaky service. The states closed, open, half-open drawn and lit. Failures count up, the breaker trips, fast-fail while open, a probe in half-open, recovery. Failure: no breaker, and the retry storm.
- ✅ `12-sharding-and-rebalance.html`: users hashed to 4 shards, a 5th added. Tab modulo hashing moves most keys, tab consistent hashing moves a slice. Then a hot shard and its fix.
- ✅ `12-latency-percentiles-and-queueing.html`: 100 requests as dots, average against p50 and p99 lines. Then a queue: utilisation steps 50, 70, 80, 90, 95 percent and the wait bar explodes past the knee. Failure: a dashboard showing only the average.
- ✅ `12-canary-and-rollback.html`: a load balancer, 10 instances. Blue-green tab: the fleet flips, instant rollback. Canary tab: 1 instance first, error rate watched, ramp 10, 50, 100 percent, or roll back at the first bad signal.

## 13. API design
- ✅ `13-idempotency-key.html`: a client POSTs a payment, the response is lost, the client retries. Without a key: two charges. With an `Idempotency-Key` header the server returns the stored response: one charge. Show the key store.
- ✅ `13-pagination-offset-vs-keyset.html`: page 1000 with `OFFSET` scans and discards 50,000 rows, keyset `WHERE id > last_id` seeks. A row inserted between pages: offset duplicates or skips one, keyset does not.

## 14. Security
- ✅ `14-oauth-code-flow.html`: browser, app, authorization server, API. Authorization code with PKCE: verifier and challenge, redirect, login, code, token exchange, access token, refresh. Failure tab: a stolen code is useless without the verifier, and a JWT with no expiry that cannot be revoked.
- ✅ `14-defence-in-depth-for-agents.html`: an agent asked to clean a disk. Rings: tool allow-list, a hook refusing `docker compose down -v`, a read-only credential, a sandbox with an empty HOME, human approval for irreversible acts. The dangerous command hits each ring. Failure: one ring only, and a persuasive prompt gets through.

## 15. Cloud and infrastructure
- ✅ `15-kubernetes-objects.html`: a Deployment makes a ReplicaSet makes 3 Pods, a Service routes by label, an Ingress routes a host, ConfigMap and Secret mounted, a PVC bound. A Pod dies and is replaced, the readiness probe gates traffic. Second tab: rolling update with maxSurge and maxUnavailable.
- ✅ `15-ab-lanes-and-switch.html`: two identical lanes A and B, each with warehouse, graph and API, a pointer naming production. Build on B, prove it, flip the pointer by a merged pull request, A becomes staging. Failure: a change on one lane only, exposed at the next flip.

## 16. Data engineering
- ✅ `16-batch-vs-stream-watermarks.html`: events with event time and arrival time, one-minute windows, a late event after the window closed, the watermark decides. Tab batch: compute once at day end. Tab stream: windows emit as the watermark passes, late data corrects.
- ✅ `16-reindex-from-genesis.html`: an indexer writes day partitions, a graph sync projects them. A decoder bug found at day 40 of 100. Tab patch rows: patched rows drift from the graph. Tab re-index: stop every writer and reader, drop, re-walk from block 1, re-sync. The rule: SQL facts are the truth, the graph is a projection.

## 17. Data structures and algorithms
- ✅ `17-big-o-ladder.html`: n steps 10, 100, 1000, 10000, 1 million. Bars for O(1), O(log n), O(n), O(n log n), O(n²), O(2ⁿ) on a log scale, a plain caption per class, a "still fine?" chip per bar.
- ✅ `17-binary-search.html`: a sorted array of 16, low, mid, high pointers, the dropped half greys out, a comparison counter. Tab linear scan for contrast. Then the analogy to index seek versus table scan.
- ✅ `17-fibonacci-three-ways.html`: tabs. Naive recursion draws the call tree for fib(6) with repeated subtrees lit and a call counter, memoised short-circuits cached nodes, iterative rolls two variables. Call counts at n=6 and the shape at n=40.
- ✅ `17-recursion-call-stack.html`: factorial(4): frames pushed, base case, frames popped with values flowing back. Failure: no base case, the stack overflows.
- ✅ `17-sorting-race.html`: one 12-element array sorted by bubble, insertion, merge and quicksort in four lanes, one comparison or swap per step, counters. End with a complexity and stability table.
- ✅ `17-hash-map-internals.html`: keys hashed into 8 buckets, a collision chain, load factor past 0.75, resize to 16 and rehash. Average O(1), worst O(n). Tie to hash join.
- ✅ `17-lru-cache-build.html`: a hash map plus a doubly linked list. get and put move a node to the front, eviction from the tail, the map entry removed. Why both structures give O(1).
- ✅ `17-bfs-vs-dfs.html`: a small graph of addresses and transfers. BFS finds the shortest hop path with a queue, DFS dives with a stack. Visit order numbers and the frontier. Tie to tracing funds from a victim to an exchange.

## 18. Model Context Protocol
- ✅ `18-mcp-lifecycle.html`: host, client, server. initialize with capabilities, initialized, tools/list, tools/call with JSON Schema arguments, the result, a resources/read, a notification, shutdown, with small JSON-RPC envelopes. Second tab: a local proxy between the host and a remote server, caching a lookup and forwarding a graph query.
- ✅ `18-mcp-tool-description-rule.html`: one tool shown to four hosts (Claude Code, Claude Desktop, Codex, a CLI). Only the tool description reaches all four. A rule in a skill reaches two, a rule in the description reaches four. Failure: a skill that missed one host for a day.

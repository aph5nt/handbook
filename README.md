# Handbook: theory for the interview

Theory for the interview, built to be understood, not memorised.

**Start here:** open [visual/index.html](visual/index.html) from your clone.
Double-click it. There are 54 animated lessons. Press Next and watch it move.

Every chapter below uses one shape for each concept:

1. a picture first
2. a short story
3. the steps that walk the picture
4. what goes wrong
5. where you use it
6. one sentence to say

The lessons and the chapters tell the same stories. The picture you watched
is the picture you read.

| # | Chapter | Rank | Core terms |
|---|---|---|---|
| 1 | [Domain-Driven Design](01-domain-driven-design.md) | ★★★ | bounded context, aggregate root, entity, value object, context map |
| 2 | [Architecture Styles](02-architecture-styles.md) | ★★★ | modular monolith, hexagonal, CQRS, event sourcing, strangler fig |
| 3 | [Distributed Systems](03-distributed-systems.md) | ★★★ | CAP, 2PC, saga, outbox, Raft, quorum, idempotency |
| 4 | [Messaging Patterns](04-messaging-patterns.md) | ★★ | delivery guarantees, partitions, DLQ, Kafka vs RabbitMQ, CDC |
| 5 | [Caching](05-caching.md) | ★★ | cache levels, cache-aside, write-through, stampede, invalidation |
| 6 | [Actor Systems](06-actor-systems.md) | ★★ | mailbox, supervision, let it crash, cluster sharding, Orleans vs Akka |
| 7 | [SQL Fundamentals](07-sql-fundamentals.md) | ★★ | join types and algorithms, window functions, isolation levels, MVCC |
| 8 | [SQL Optimisation](08-sql-optimisation.md) | ★★★ | seek vs scan, EXPLAIN, covering index, partition pruning, SARGable |
| 9 | [Agentic Design Patterns](09-agentic-design-patterns.md) | ★★★ | ReAct, LangGraph, RAG, MCP, supervisor, evals, guardrails |
| 10 | [LLM Systems](10-llm-systems.md) | ★★ | tokens, embeddings, KV cache, fine-tune vs RAG, non-determinism |
| 11 | [Famous Architectures](11-famous-architectures.md) | ★★ | Netflix, Spotify, Uber, Amazon, Google, Discord, Shopify |
| 12 | [Scalability and Reliability](12-scalability-and-reliability.md) | ★★★ | sharding, circuit breaker, p99, SLO, error budget, chaos |
| 13 | [API Design](13-api-design.md) | ★ | REST vs GraphQL vs gRPC, versioning, idempotency keys, webhooks |
| 14 | [Security](14-security.md) | ★ | OAuth 2.0, OIDC, JWT, zero trust, OWASP, agent sandboxing |
| 15 | [Cloud and Infrastructure](15-cloud-and-infrastructure.md) | ★ | Kubernetes objects, Helm, GitOps, service mesh, twelve-factor |
| 16 | [Data Engineering](16-data-engineering.md) | ★★ | lakehouse, star schema, Parquet, CDC, watermarks, reprocessing |
| 17 | [Data Structures and Algorithms](17-data-structures-and-algorithms.md) | ★★ | Big-O ladder, N+1, recursion, Fibonacci three ways, sorts, hash map, LRU |
| 18 | [Model Context Protocol](18-model-context-protocol.md) | ★★★ | host, client, server, tools, resources, prompts, sampling, roots, security |

★★★ must master · ★★ likely asked · ★ be ready

[TEMPLATE.md](TEMPLATE.md) is the shape every chapter follows. Add a
chapter by copying it.

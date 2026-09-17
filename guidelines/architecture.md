# Architecture rules

- Webhook handlers must acknowledge GitHub quickly (HTTP 202) and process reviews asynchronously.
- Isolate agent failures: a linter timeout must not drop security findings.
- Persist token usage, latency, and estimated cost per LLM call and per PR, even when the model is free.
- RAG results are hints, not source of truth. If no guideline matches, say so instead of inventing policy.
- Review comments on GitHub should be a single aggregated markdown report, not one comment per agent.
- Idempotency: the same PR delivery id should not enqueue two full reviews.

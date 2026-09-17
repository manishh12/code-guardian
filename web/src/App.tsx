import { useEffect, useState } from "react";

type Totals = {
  review_count: number;
  tokens: number;
  avg_latency_ms: number;
  actual_cost_usd: number;
  equivalent_paid_usd: number;
};

type Review = {
  id: string;
  repo: string;
  pr_number: number;
  status: string;
  provider: string;
  model: string;
  tokens_in: number;
  tokens_out: number;
  latency_ms: number;
  estimated_cost_usd: string;
  equivalent_paid_usd: string;
  created_at: string;
  error: string | null;
};

type AgentRow = { agent: string; tokens: number; avg_latency_ms: number };

type Metrics = {
  provider: { provider: string; model: string };
  totals: Totals;
  reviews: Review[];
  byAgent: AgentRow[];
};

const money = (value: number | string) => `$${Number(value).toFixed(4)}`;

export function App() {
  const [data, setData] = useState<Metrics | null>(null);
  const [busy, setBusy] = useState(false);
  const [comment, setComment] = useState("");
  const [error, setError] = useState("");

  async function load() {
    const response = await fetch("/api/metrics");
    if (!response.ok) throw new Error("API is not reachable. Start the server.");
    setData(await response.json());
  }

  useEffect(() => {
    load().catch((err: Error) => setError(err.message));
  }, []);

  async function runDemo() {
    setBusy(true);
    setError("");
    try {
      const response = await fetch("/webhooks/demo", { method: "POST" });
      const body = await response.json();
      if (!response.ok) throw new Error(body.error ?? "demo failed");
      setComment(body.comment ?? "");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  const totals = data?.totals;

  return (
    <div className="page">
      <div className="hero">
        <div>
          <h1>Code Guardian</h1>
          <p className="muted">
            Multi-agent PR review · RAG guidelines · token / latency / cost guardrails
            {data ? ` · ${data.provider.provider}/${data.provider.model}` : ""}
          </p>
        </div>
        <button onClick={runDemo} disabled={busy}>
          {busy ? "Reviewing…" : "Run demo review"}
        </button>
      </div>

      {error ? <p className="bad">{error}</p> : null}

      <div className="row">
        <div className="card">
          <h2>Reviews</h2>
          <p>{totals?.review_count ?? 0}</p>
        </div>
        <div className="card">
          <h2>Tokens</h2>
          <p>{totals?.tokens ?? 0}</p>
        </div>
        <div className="card">
          <h2>Avg latency</h2>
          <p>{totals?.avg_latency_ms ?? 0} ms</p>
        </div>
        <div className="card">
          <h2>Free spend / paid equivalent</h2>
          <p>
            {money(totals?.actual_cost_usd ?? 0)} / {money(totals?.equivalent_paid_usd ?? 0)}
          </p>
        </div>
      </div>

      <div className="card" style={{ marginBottom: 16 }}>
        <h2>Per-agent usage</h2>
        <table>
          <thead>
            <tr>
              <th>Agent</th>
              <th>Tokens</th>
              <th>Avg latency</th>
            </tr>
          </thead>
          <tbody>
            {(data?.byAgent ?? []).map((row) => (
              <tr key={row.agent}>
                <td>{row.agent}</td>
                <td>{row.tokens}</td>
                <td>{row.avg_latency_ms} ms</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="card">
        <h2>Recent PR reviews</h2>
        <table>
          <thead>
            <tr>
              <th>When</th>
              <th>Repo / PR</th>
              <th>Status</th>
              <th>Tokens</th>
              <th>Latency</th>
              <th>Paid equivalent</th>
            </tr>
          </thead>
          <tbody>
            {(data?.reviews ?? []).map((review) => (
              <tr key={review.id}>
                <td>{new Date(review.created_at).toLocaleString()}</td>
                <td>
                  {review.repo}#{review.pr_number}
                </td>
                <td>
                  <span className={`pill ${review.status === "completed" ? "ok" : "bad"}`}>{review.status}</span>
                </td>
                <td>{review.tokens_in + review.tokens_out}</td>
                <td>{review.latency_ms} ms</td>
                <td>{money(review.equivalent_paid_usd)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {comment ? (
        <div className="card" style={{ marginTop: 16 }}>
          <h2>Last GitHub comment</h2>
          <pre>{comment}</pre>
        </div>
      ) : null}
    </div>
  );
}

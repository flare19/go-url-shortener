import { useState } from "react";
import { getStats, type StatsResponse } from "../api";

export default function StatsPage() {
  const [code, setCode] = useState("");
  const [result, setResult] = useState<StatsResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setResult(null);
    setLoading(true);
    try {
      const res = await getStats(code.trim());
      setResult(res);
    } catch (err) {
      setError(err instanceof Error ? err.message : "something went wrong");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="max-w-xl mx-auto px-6 py-16">
      <h1 className="font-mono text-sm text-zinc-500 mb-1">look up stats</h1>
      <p className="text-zinc-400 text-sm mb-8">
        enter a short code to see hit count and creation time.
      </p>

      <form onSubmit={handleSubmit} className="flex gap-2">
        <input
          type="text"
          required
          placeholder="TcocgMn"
          value={code}
          onChange={(e) => setCode(e.target.value)}
          className="flex-1 bg-panel border border-edge rounded px-3 py-2 font-mono text-sm text-zinc-200 placeholder:text-zinc-600 focus:outline-none focus:border-accent"
        />
        <button
          type="submit"
          disabled={loading}
          className="px-4 py-2 rounded bg-accentdim text-zinc-900 font-mono text-sm font-medium hover:bg-accent disabled:opacity-50 transition-colors"
        >
          {loading ? "..." : "lookup"}
        </button>
      </form>

      {error && (
        <p className="mt-4 font-mono text-sm text-red-400">error: {error}</p>
      )}

      {result && (
        <div className="mt-6 bg-panel border border-edge rounded px-4 py-4">
          <p className="font-mono text-xs text-zinc-500 mb-3 break-all">
            {result.long_url}
          </p>
          <div className="flex items-end justify-between">
            <div>
              <p className="text-[10px] uppercase tracking-wide text-zinc-500 font-mono">
                hits
              </p>
              <p className="text-3xl font-mono text-accent tabular-nums">
                {result.hit_count}
              </p>
            </div>
            <p className="text-xs text-zinc-500 font-mono">
              created {new Date(result.created_at).toLocaleString()}
            </p>
          </div>
        </div>
      )}
    </div>
  );
}
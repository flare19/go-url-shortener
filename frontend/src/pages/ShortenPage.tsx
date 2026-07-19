import { useState } from "react";
import { shorten, type ShortenResponse } from "../api";

export default function ShortenPage() {
  const [longUrl, setLongUrl] = useState("");
  const [result, setResult] = useState<ShortenResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [copied, setCopied] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setResult(null);
    setCopied(false);
    setLoading(true);
    try {
      const res = await shorten(longUrl);
      setResult(res);
    } catch (err) {
      setError(err instanceof Error ? err.message : "something went wrong");
    } finally {
      setLoading(false);
    }
  }

  function handleCopy() {
    if (!result) return;
    navigator.clipboard.writeText(result.short_url);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  }

  return (
    <div className="max-w-xl mx-auto px-6 py-16">
      <h1 className="font-mono text-sm text-zinc-500 mb-1">shorten a url</h1>
      <p className="text-zinc-400 text-sm mb-8">
        no accounts, no history — copy your link before you navigate away.
      </p>

      <form onSubmit={handleSubmit} className="flex gap-2">
        <input
          type="url"
          required
          placeholder="https://example.com/some/long/path"
          value={longUrl}
          onChange={(e) => setLongUrl(e.target.value)}
          className="flex-1 bg-panel border border-edge rounded px-3 py-2 font-mono text-sm text-zinc-200 placeholder:text-zinc-600 focus:outline-none focus:border-accent"
        />
        <button
          type="submit"
          disabled={loading}
          className="px-4 py-2 rounded bg-accentdim text-zinc-900 font-mono text-sm font-medium hover:bg-accent disabled:opacity-50 transition-colors"
        >
          {loading ? "..." : "shorten"}
        </button>
      </form>

      {error && (
        <p className="mt-4 font-mono text-sm text-red-400">error: {error}</p>
      )}

      {result && (
        <div className="mt-6 bg-panel border border-edge rounded px-4 py-3">
          <div className="flex items-center justify-between gap-3">
            <code className="font-mono text-sm text-accent break-all">
              {result.short_url}
            </code>
            <button
              onClick={handleCopy}
              className="shrink-0 px-2.5 py-1 rounded border border-edge text-xs font-mono text-zinc-300 hover:border-accent hover:text-accent transition-colors"
            >
              {copied ? "copied" : "copy"}
            </button>
          </div>
          <p className="mt-2 text-xs text-zinc-500 font-mono">
            → {result.long_url}
          </p>
        </div>
      )}
    </div>
  );
}
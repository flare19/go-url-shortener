const WRITER_URL = import.meta.env.VITE_WRITER_URL;
const REDIRECTOR_URL = import.meta.env.VITE_REDIRECTOR_URL;

export interface ShortenResponse {
  code: string;
  short_url: string;
  long_url: string;
  created_at: string;
}

export interface StatsResponse {
  code: string;
  long_url: string;
  hit_count: number;
  created_at: string;
}

export interface ApiError {
  error: string;
}

export async function shorten(longUrl: string): Promise<ShortenResponse> {
  const res = await fetch(`${WRITER_URL}/shorten`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ long_url: longUrl }),
  });
  if (!res.ok) {
    const body: ApiError = await res.json().catch(() => ({ error: "request failed" }));
    throw new Error(body.error);
  }
  return res.json();
}

export async function getStats(code: string): Promise<StatsResponse> {
  const res = await fetch(`${REDIRECTOR_URL}/${encodeURIComponent(code)}/stats`);
  if (!res.ok) {
    const body: ApiError = await res.json().catch(() => ({ error: "request failed" }));
    throw new Error(body.error);
  }
  return res.json();
}
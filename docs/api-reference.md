# API Reference

## POST /shorten

Create a short code for a long URL.

**Request**
```json
{
  "long_url": "https://example.com/some/very/long/path"
}
```

**Response** `201 Created`
```json
{
  "code": "aZ3kT9",
  "short_url": "http://localhost:PORT/aZ3kT9",
  "long_url": "https://example.com/some/very/long/path",
  "created_at": "2026-07-18T13:29:08Z"
}
```

**Errors**
| Status | Condition |
|--------|-----------|
| 400    | invalid/malformed URL |
|        | _(fill in others as implemented, e.g. rate limit if added later)_ |

---

## GET /{code}

Redirect to the original long URL.

**Response**
- `302 Found` with `Location` header set to the original URL
- `404 Not Found` if code doesn't exist

---

## GET /{code}/stats

Return hit count for a short code.

**Response** `200 OK`
```json
{
  "code": "aZ3kT9",
  "long_url": "https://example.com/some/very/long/path",
  "hit_count": 0,
  "created_at": ""
}
```

**Errors**
| Status | Condition |
|--------|-----------|
| 404    | code doesn't exist |
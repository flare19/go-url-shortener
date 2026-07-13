# Database Schema

## Collections

### `urls`

| Field       | Type      | Notes                                  |
|-------------|-----------|------------------------------------------|
| `_id`       |           |                                          |
| `code`      |           | short code, indexed, unique             |
| `long_url`  |           |                                          |
| `created_at`|           |                                          |
| `hit_count` |           |                                          |

_(fill in actual BSON types once the domain struct is written)_

### `counters` _(only if counter-based generation is chosen — see ADR 0001)_

| Field   | Type | Notes                        |
|---------|------|-------------------------------|
| `_id`   |      | e.g. `"url_code"`             |
| `seq`   |      | current counter value         |

## Indexes

- `urls.code` — unique index, this is the hot lookup path for redirects

## Notes

- Fill in TTL / expiry policy here if one gets added later.
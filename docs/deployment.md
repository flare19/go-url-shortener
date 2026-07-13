# Deployment

## Local

```bash
docker-compose up
```

_(fill in once docker-compose.yml exists — services, ports, env vars)_

## Environment variables

| Variable      | Service     | Description               | Default |
|---------------|-------------|----------------------------|---------|
| `MONGO_URI`   | both        | Mongo connection string    |         |
| `WRITER_PORT` | writer      |                             |         |
| `REDIRECT_PORT`| redirector |                             |         |

## Production

_(fill in if/when this goes beyond local — target platform, e.g. Railway/
Render/EC2, following the pattern from Invitrack/Collabify if reused)_

## CI/CD

_(fill in if a GitHub Actions workflow gets added — lint, test, build steps)_
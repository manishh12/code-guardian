# Go style

- Prefer explicit error returns; wrap with `%w` and include repo/PR context at API boundaries.
- Do not panic in request handlers. Log and return HTTP errors.
- Keep webhook handlers fast: respond 202, run reviews in a goroutine.
- Cap concurrency when fetching many PR files.
- Secrets stay in environment variables. Never commit `.env`.
- Prefer `pgx` parameterized queries; never concatenate SQL from user input.

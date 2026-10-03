# TheRAGService

A REST API service written in Go, providing JWT-based authentication and management of RAG collections.

## Features

- **User registration** — passwords hashed with bcrypt
- **Login** — HS256-signed JWT access token + refresh token
- **Token refresh** — refresh token rotation; the old token is deleted and the new one is stored in an atomic transaction
- **Secure token storage** — refresh tokens are kept in the database as SHA-256 hashes
- **JWT auth middleware** — protected routes require a valid `Bearer` access token
- **Collections (RAG)** — per-user collection management; collection names are unique per user
- **IP-based rate limiting** — Redis-backed, configurable fixed window per endpoint
- **Automatic table creation** — `users`, `refresh_tokens` and `collections` tables are prepared on service startup

## Tech Stack

| Component | Purpose |
| --- | --- |
| [Go](https://go.dev) 1.27+ | Main language |
| [chi](https://github.com/go-chi/chi) v5 | HTTP router |
| [pgx](https://github.com/jackc/pgx) v5 | PostgreSQL driver |
| [go-redis](https://github.com/redis/go-redis) v9 | Redis client |
| [golang-jwt](https://github.com/golang-jwt/jwt) v5 | JWT creation/verification |
| [bcrypt](https://pkg.go.dev/golang.org/x/crypto/bcrypt) | Password hashing |
| [godotenv](https://github.com/joho/godotenv) | `.env` loading |

## Getting Started

### Requirements

- Go 1.27 or later
- PostgreSQL (running)
- Redis (running)

### Steps

```bash
# 1. Clone the repository
git clone <repo-url>
cd TheRAGService

# 2. Download dependencies
go mod download

# 3. Configure environment variables (see below)
cp .env.example .env   # if an example file exists
# or create the .env file manually

# 4. Run the service
go run .
```

The service listens on port **`:8080`** by default.

## Environment Variables

The `.env` file must be located in the project root:

| Variable | Required | Description |
| --- | --- | --- |
| `DB_URL` | ✅ | PostgreSQL connection string, e.g. `postgres://user:pass@localhost:5432/dbname` |
| `JWT_SECRET` | ✅ | JWT signing key (use a strong, random value) |

Example `.env`:

```dotenv
DB_URL=postgres://postgres:postgres@localhost:5432/ragservice
JWT_SECRET=change-me-to-a-long-random-string
```

> Redis connection details are currently hardcoded in `internal/store/redis.go` (`localhost:6379`).

## API Endpoints

All endpoints are mounted under `http://localhost:8080/api/v1`.

### Auth

| Method | Path | Rate Limit | Description |
| --- | --- | --- | --- |
| `POST` | `/auth/register` | 3 / minute | Create a new user |
| `POST` | `/auth/login` | 5 / minute | Log in; returns access + refresh tokens |
| `POST` | `/auth/refresh` | 5 / minute | Exchange an unexpired refresh token for a new token pair |

### Collections (protected)

These endpoints require a valid access token:

```
Authorization: Bearer <access_token>
```

| Method | Path | Rate Limit | Description |
| --- | --- | --- | --- |
| `POST` | `/collection` | 10 / minute | Create a new collection |
| `GET` | `/collection` | 30 / minute | List the current user's collections |
| `DELETE` | `/collection/{collection_id}` | 10 / minute | Delete one of the current user's collections |

### `POST /auth/register`

```json
// Request
{
  "username": "coderian",
  "email": "coderian@example.com",
  "password": "supersecret"
}

// Response (201 Created)
{
  "id": 1,
  "username": "coderian",
  "email": "coderian@example.com"
}
```

### `POST /auth/login`

```json
// Request
{
  "username": "coderian",
  "password": "supersecret"
}

// Response (200 OK)
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "Yl9k3xR2..."
}
```

### `POST /auth/refresh`

```json
// Request
{
  "refresh_token": "Yl9k3xR2..."
}

// Response (200 OK)
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "Qm4pL8zV..."
}
```

### `POST /collection`

```json
// Request
{
  "name": "my-collection",
  "description": "Notes about RAG"
}

// Response (201 Created)
{
  "message": "Collection created"
}
```

### `GET /collection`

```json
// Response (200 OK)
{
  "collections": [
    {
      "id": 1,
      "name": "my-collection",
      "description": "Notes about RAG",
      "created_at": "2026-10-03T12:00:00Z"
    }
  ]
}
```

### `DELETE /collection/{collection_id}`

```json
// Response (200 OK)
{
  "message": "Collection deleted"
}
```

### Error Responses

All errors are returned in the following format:

```json
{ "error": "invalid credentials" }
```

| Status | Meaning |
| --- | --- |
| `400 Bad Request` | Invalid JSON body, missing field or invalid collection ID |
| `401 Unauthorized` | Wrong credentials / missing, invalid or expired token |
| `404 Not Found` | Collection does not exist or belongs to another user |
| `409 Conflict` | Username or email already registered / collection name already exists |
| `429 Too Many Requests` | Rate limit exceeded (see the `Retry-After` header) |
| `500 Internal Server Error` | Server-side error |

## Token Lifecycle

- **Access token:** HS256-signed JWT, valid for **30 minutes** (`tokenTTL`)
- **Refresh token:** 32 bytes of cryptographically secure random data; valid for **30 days** (`refreshTokenTTL`)
- Refresh tokens are stored in the database as SHA-256 hashes — even if the database leaks, the raw tokens cannot be recovered
- Every `/auth/refresh` call performs **rotation**: the old token is deleted and the new one is written within the same transaction
- An expired refresh token is removed from the database at request time and the call returns `401`
- The `Auth` middleware validates the `Bearer` access token on every protected request, accepting **HS256 only** (prevents algorithm-confusion attacks)

## Project Structure

```
.
├── main.go                          # Entry point: env, DB connections, router
├── go.mod
├── LICENSE
├── README.md
└── internal/
    ├── handlers/
    │   ├── auth/
    │   │   └── handlers.go          # Register, Login, Refresh handlers
    │   └── rag/
    │       └── handlers.go          # Collection create/list/delete handlers
    ├── helpers/
    │   └── random.go                # Cryptographically secure random string generator
    ├── httpx/
    │   └── response.go              # Shared JSON response helpers (WriteJSON, WriteError)
    ├── middlewares/
    │   ├── auth.go                  # JWT verification middleware
    │   └── rate_limit.go            # Redis-backed IP-based rate limiter
    ├── models/
    │   ├── auth.go                  # Auth request/response models, JWT Claims
    │   └── rag.go                   # Collection request/response models
    └── store/
        ├── postre.go                # PostgreSQL connection and table creation
        └── redis.go                 # Redis connection
```

## Database Schema

```sql
CREATE TABLE users (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username   TEXT NOT NULL UNIQUE,
    email      TEXT NOT NULL UNIQUE,
    password   TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE refresh_tokens (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token      TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE collections (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    collection_name TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, collection_name)
);
```

> The `UNIQUE (user_id, collection_name)` constraint guarantees at the database level that a user cannot create two collections with the same name, even under concurrent requests.

## License

This project is licensed under the [MIT License](LICENSE).

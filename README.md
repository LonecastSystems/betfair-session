# betfair-session

Go library for managing [Betfair](https://www.betfair.com/) API session tokens with cloud-agnostic blob storage ([Go Cloud](https://gocloud.dev/blob/)).

Persists the session token, resumes it when still valid, and logs in again when missing or expired. Swap storage backends (local disk, Azure, AWS, GCP) without changing application code.

## Prerequisites

- Go 1.25.0+
- [betfair-go](https://github.com/LonecastSystems/betfair-go) for Betfair API login, resume, and logout (installed automatically as a dependency)
- A valid Betfair account with certificate login enabled
- Betfair application key
- Client certificate and private key (PEM)
- A `gocloud.dev/blob` bucket (local directory for dev, Azure Blob / S3 / GCS in production)

## Usage

Install:

```bash
go get github.com/LonecastSystems/betfair-session
```

Import the package:

```go
import (
    "context"

    "github.com/LonecastSystems/betfair-go"
    "github.com/LonecastSystems/betfair-session"
    "gocloud.dev/blob/fileblob"
)
```

Basic flow:

```go
bucket, err := fileblob.OpenBucket("./session-data", &fileblob.Options{
    NoTempDir: true, // required on Windows if the bucket is not on the same drive as %TEMP%
})
if err != nil {
    // handle error
}
defer bucket.Close()

tlsConfig, err := betfair.GetTLSConfigFromBytes(certPEM, keyPEM)
if err != nil {
    // handle error
}

client := betfair.NewClient(&betfair.ClientConfig{
    Tls:            tlsConfig,
    ApplicationKey: appKey,
})

mgr := session.NewSessionManager(client, username, password)

token, err := mgr.GetSession(context.Background(), bucket, "session.token")
if err != nil {
    // handle error
}

_ = token
```

## Session management

`GetSession` is **idempotent**: you can call it before every API request (or on a schedule) without creating a new login each time. If a stored token is still valid, it is resumed and returned; a new login only runs when the token is missing, empty, or no longer accepted by Betfair.

This follows [Betfair’s recommended session management](https://support.developer.betfair.com/hc/en-us/articles/11775498092701-Why-I-am-received-the-error-INVALID-SESSION-INFORMATION):

- Use **one session** across multiple API calls and goroutines — do not log in on every request.
- When a session expires, obtain a new session token via login (or resume when the stored token is still valid).
- Handle **`INVALID_SESSION_INFORMATION`** in your application: call `GetSession` again (or retry the API call after refreshing the session) when Betfair returns that error.

Betfair sessions last up to **12 hours** by default; the Keep Alive API can extend an active session. Excessive logins can trigger a **20-minute** login lockout — reusing a valid session avoids that.

## How it works

1. Checks that the bucket is accessible.
2. If `sessionKey` exists in the bucket, reads the token and calls `Resume` on the Betfair client.
3. If resume fails or the object is missing, calls `Login` and writes the new token with `WriteAll`.
4. If writing the token fails after login, calls `Logout` on the client.

## Storage backends

Storage is cloud-agnostic: any [Go Cloud blob URL](https://gocloud.dev/howto/blob/) works, for example:

| Environment | Example |
|-------------|---------|
| Local dev | `fileblob.OpenBucket(dir, &fileblob.Options{NoTempDir: true})` |
| Azure | `azblob://mycontainer` (via `blob.OpenBucket`) |
| AWS | `s3://mybucket` |
| GCP | `gs://mybucket` |

Store only the **session token** in the bucket. Keep username, password, and certificates in a secrets manager (e.g. Azure Key Vault), not in blob storage.

## Notes

- Keep credentials, application keys, and certificate paths out of source code. Read them from environment variables or your secrets manager.
- On Windows, if the bucket path is on a different drive than `%TEMP%`, set `fileblob.Options.NoTempDir: true` or the final rename on write can fail.
- Call `GetSession` before API use (or when you see `INVALID_SESSION_INFORMATION`); the token is cached on the `betfair.Client` after `Resume` / `Login`.
- Restrict bucket access: the session token is as sensitive as login credentials for API purposes.
- Depends on [betfair-go](https://github.com/LonecastSystems/betfair-go) for login, resume, and logout.

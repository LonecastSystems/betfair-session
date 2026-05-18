package session

import (
	"context"
	"errors"

	"github.com/LonecastSystems/betfair-go"
	"gocloud.dev/blob"
)

// SessionManager holds a Betfair client and credentials used to obtain session tokens.
type SessionManager struct {
	client   *betfair.Client
	username string
	password string
}

// NewSessionManager returns a manager that uses client for Login, Resume, and Logout.
// username and password are required when a new login is needed.
func NewSessionManager(client *betfair.Client, username string, password string) *SessionManager {
	return &SessionManager{client: client, username: username, password: password}
}

// GetSession returns a valid Betfair session token, reusing a stored token when possible.
//
// It is idempotent: call it before API requests without logging in each time. If sessionKey
// exists in bucket, the stored token is resumed when still valid; otherwise Login runs and
// the new token is written to bucket. On write failure after login, the client is logged out.
//
// sessionKey is the blob object name (for example "session.token"). bucket must be accessible.
func (s *SessionManager) GetSession(ctx context.Context, bucket *blob.Bucket, sessionKey string) (string, error) {
	accessible, err := bucket.IsAccessible(ctx)
	if err != nil {
		return "", err
	}
	if !accessible {
		return "", errors.New("session bucket is not accessible")
	}

	if sessionKey == "" {
		return "", errors.New("sessionKey is required")
	}

	// Check if the session token is already in the bucket
	exists, err := bucket.Exists(ctx, sessionKey)
	if err != nil {
		return "", err
	}

	// If the session token exists, try to resume it
	if exists {
		sessionTokenBytes, err := bucket.ReadAll(ctx, sessionKey)
		if err != nil {
			return "", err
		}

		// If the session token is not empty, try to resume it
		sessionToken := string(sessionTokenBytes)
		if sessionToken != "" {
			if _, err := s.client.Resume(ctx, sessionToken); err == nil {
				// Session token is valid, return it
				return sessionToken, nil
			}
		}

		// If the resume fails, continue to login
	}

	// Login if the session doesnt exist or it's not valid
	if s.username == "" || s.password == "" {
		return "", errors.New("username and password are required")
	}

	resp, err := s.client.Login(ctx, s.username, s.password)
	if err != nil {
		return "", err
	}

	// Write the session token to the bucket
	err = bucket.WriteAll(ctx, sessionKey, []byte(resp.SessionToken), nil)
	if err != nil {
		if _, err := s.client.Logout(ctx); err != nil {
			return "", err
		}

		return "", err
	}

	return resp.SessionToken, nil
}

package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// tokenStore keeps refresh tokens, the access-token denylist, the per-user
// permission cache and login-failure counters in Redis.
type tokenStore struct {
	rdb *redis.Client
}

type refreshRecord struct {
	UserID   uuid.UUID `json:"uid"`
	TenantID uuid.UUID `json:"tid"`
	Issued   time.Time `json:"iat"`
}

func keyRefresh(t string) string        { return "auth:refresh:" + t }
func keyDeny(jti string) string         { return "auth:deny:" + jti }
func keyPerms(uid uuid.UUID) string     { return "auth:perms:" + uid.String() }
func keyFail(user, ip string) string    { return "auth:fail:" + user + ":" + ip }
func keyUserRefresh(uid uuid.UUID) string { return "auth:user-refresh:" + uid.String() }

func newOpaque() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *tokenStore) issueRefresh(ctx context.Context, uid, tid uuid.UUID, ttl time.Duration) (string, error) {
	tok, err := newOpaque()
	if err != nil {
		return "", err
	}
	raw, _ := json.Marshal(refreshRecord{UserID: uid, TenantID: tid, Issued: time.Now()})
	pipe := s.rdb.TxPipeline()
	pipe.Set(ctx, keyRefresh(tok), raw, ttl)
	pipe.SAdd(ctx, keyUserRefresh(uid), tok)
	pipe.Expire(ctx, keyUserRefresh(uid), ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", fmt.Errorf("store refresh token: %w", err)
	}
	return tok, nil
}

var errRefreshInvalid = errors.New("refresh token invalid or expired")

// consumeRefresh atomically deletes and returns the record (rotation).
func (s *tokenStore) consumeRefresh(ctx context.Context, tok string) (*refreshRecord, error) {
	raw, err := s.rdb.GetDel(ctx, keyRefresh(tok)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, errRefreshInvalid
	}
	if err != nil {
		return nil, err
	}
	var rec refreshRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return nil, errRefreshInvalid
	}
	s.rdb.SRem(ctx, keyUserRefresh(rec.UserID), tok)
	return &rec, nil
}

// revokeAllRefresh drops every refresh token of a user (password change, disable, role change).
func (s *tokenStore) revokeAllRefresh(ctx context.Context, uid uuid.UUID) error {
	toks, err := s.rdb.SMembers(ctx, keyUserRefresh(uid)).Result()
	if err != nil {
		return err
	}
	pipe := s.rdb.Pipeline()
	for _, t := range toks {
		pipe.Del(ctx, keyRefresh(t))
	}
	pipe.Del(ctx, keyUserRefresh(uid))
	_, err = pipe.Exec(ctx)
	return err
}

func (s *tokenStore) deny(ctx context.Context, jti string, until time.Time) error {
	ttl := time.Until(until)
	if ttl <= 0 {
		return nil
	}
	return s.rdb.Set(ctx, keyDeny(jti), 1, ttl).Err()
}

func (s *tokenStore) isDenied(ctx context.Context, jti string) (bool, error) {
	n, err := s.rdb.Exists(ctx, keyDeny(jti)).Result()
	return n > 0, err
}

const permsCacheTTL = 5 * time.Minute

// cachedUser is the per-user snapshot the auth middleware needs on every request.
type cachedUser struct {
	TenantID uuid.UUID `json:"tid"`
	Username string    `json:"usr"`
	Status   string    `json:"st"`
	IsSuper  bool      `json:"sup"`
	Perms    []string  `json:"perms"`
}

func (s *tokenStore) getCachedUser(ctx context.Context, uid uuid.UUID) (*cachedUser, error) {
	raw, err := s.rdb.Get(ctx, keyPerms(uid)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var cu cachedUser
	if err := json.Unmarshal(raw, &cu); err != nil {
		return nil, nil
	}
	return &cu, nil
}

func (s *tokenStore) setCachedUser(ctx context.Context, uid uuid.UUID, cu *cachedUser) error {
	raw, _ := json.Marshal(cu)
	return s.rdb.Set(ctx, keyPerms(uid), raw, permsCacheTTL).Err()
}

// InvalidatePerms drops the cached permission set; call after role/permission changes.
func (s *tokenStore) invalidatePerms(ctx context.Context, uids ...uuid.UUID) error {
	if len(uids) == 0 {
		return nil
	}
	keys := make([]string, len(uids))
	for i, u := range uids {
		keys[i] = keyPerms(u)
	}
	return s.rdb.Del(ctx, keys...).Err()
}

const (
	loginMaxFailures = 5
	loginLockWindow  = 15 * time.Minute
)

// recordFailure increments the failure counter and reports whether the account is now locked.
func (s *tokenStore) recordFailure(ctx context.Context, user, ip string) (locked bool, err error) {
	k := keyFail(user, ip)
	n, err := s.rdb.Incr(ctx, k).Result()
	if err != nil {
		return false, err
	}
	if n == 1 {
		s.rdb.Expire(ctx, k, loginLockWindow)
	}
	return n >= loginMaxFailures, nil
}

func (s *tokenStore) isLocked(ctx context.Context, user, ip string) (bool, error) {
	n, err := s.rdb.Get(ctx, keyFail(user, ip)).Int64()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return n >= loginMaxFailures, nil
}

func (s *tokenStore) clearFailures(ctx context.Context, user, ip string) {
	s.rdb.Del(ctx, keyFail(user, ip))
}

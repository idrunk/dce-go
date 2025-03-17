package redises

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.drunkce.com/dce/session"
	"go.drunkce.com/dce/util"
)

// Session is a generic struct that represents a session in a Redis-backed session management system.
// It combines functionality from BasicSession, UserSession, and ConnectionSession to provide
// a comprehensive session management solution. The struct is parameterized with a type U that
// implements the UidGetter interface, allowing it to work with any user type that provides a UID.
//
// The Session struct holds references to:
// - BasicSession: Manages basic session properties like session ID and TTL.
// - UserSession: Manages user-specific session data and operations.
// - ConnectionSession: Manages connection-related session data and operations.
// - redis: A Redis client used to interact with the Redis database.
// - ctx: A context.Context used for Redis operations.
//
// This struct provides methods for session manipulation, including setting, getting, and deleting
// session fields, as well as more complex operations like session renewal, cloning, and synchronization
// with user data.
type Session[U session.UidGetter] struct {
	*session.BasicSession
	*session.UserSession[U]
	*session.ConnectionSession
	redis *redis.Client
	ctx   context.Context
}

func NewSession[U session.UidGetter](rdb *redis.Client, sidPool []string, ttlMinutes uint16) (*Session[U], error) {
	basic, err := session.NewBasicSession(sidPool, ttlMinutes)
	if err != nil {
		return nil, err
	}
	rs := &Session[U]{
		BasicSession:      basic,
		UserSession:       session.NewUserSession[U](basic),
		ConnectionSession: session.NewConnectionSession(basic),
		redis:             rdb,
		ctx:               context.Background(),
	}
	rs.BasicSession.IfSession = rs
	rs.UserSession.IfUserSession = rs
	rs.ConnectionSession.IfConnection = rs
	return rs, nil
}

func redisGenKey(prefix string, id string) string {
	return fmt.Sprintf("%s:%s", prefix, id)
}

func (r *Session[U]) Key() string {
	return redisGenKey(r.SidName, r.BasicSession.Id())
}

func (r *Session[U]) SilentSet(field string, value any) error {
	return r.redis.HSet(r.ctx, r.Key(), field, value).Err()
}

func (r *Session[U]) SilentGet(field string) (any, error) {
	return r.redis.HGet(r.ctx, r.Key(), field).Result()
}

func (r *Session[U]) SilentDel(field string) error {
	return r.redis.HDel(r.ctx, r.Key(), field).Err()
}

func (r *Session[U]) Destroy() error {
	if err := r.Unmapping(); err != nil {
		return err
	}
	return r.redis.Del(r.ctx, r.Key()).Err()
}

func (r *Session[U]) Touch() error {
	return r.redis.Expire(r.ctx, r.Key(), time.Duration(r.TtlSeconds())*time.Second).Err()
}

func (r *Session[U]) Load(data map[string]any) error {
	return r.redis.HSet(r.ctx, r.Key(), data).Err()
}

func (r *Session[U]) Raw() (map[string]any, error) {
	data, err := r.redis.HGetAll(r.ctx, r.Key()).Result()
	if err != nil {
		return nil, err
	}
	result := make(map[string]any)
	for k, v := range data {
		result[k] = v
	}
	return result, nil
}

func (r *Session[U]) TtlPassed() (uint32, error) {
	ttl, err := r.redis.TTL(r.ctx, r.Key()).Result()
	if err != nil {
		return 0, err
	} else if ttl.Seconds() < 1 {
		return 0, util.Closed0("ttl was not initialized yet.")
	}
	return r.TtlSeconds() - uint32(ttl.Seconds()), nil
}

func (r *Session[U]) Renew(filters map[string]any) error {
	err := r.UserSession.Renew(filters)
	if err == nil && r.Request() {
		return r.UpdateShadow(r.BasicSession.Id())
	}
	return err
}

func (r *Session[U]) Clone(id string) (any, error) {
	cloned := *r
	cl := &cloned
	basic, err := r.CloneBasic(cl, id)
	if err != nil {
		return nil, err
	}
	cl.BasicSession = basic
	cl.UserSession = r.CloneUser(cl, basic)
	cl.ConnectionSession = r.CloneConnection(cl, basic)
	return cl, nil
}

func redisGenUserKey(userPrefix string, id uint64) string {
	return fmt.Sprintf("%s:%d", userPrefix, id)
}

func (r *Session[U]) UserKey() (string, error) {
	uid, err := r.Uid()
	if err != nil {
		return "", err
	}
	return redisGenUserKey(r.KeyPrefix, uid), nil
}

func (r *Session[U]) Mapping() error {
	userKey, err := r.UserKey()
	if err != nil {
		return err
	}
	pipe := r.redis.Pipeline()
	pipe.SAdd(r.ctx, userKey, r.BasicSession.Id())
	pipe.Expire(r.ctx, userKey, time.Duration(session.MappingTtlSeconds)*time.Second)
	_, err = pipe.Exec(r.ctx)
	return err
}

func (r *Session[U]) Unmapping() error {
	userKey, err := r.UserKey()
	if err != nil {
		return err
	}
	return r.redis.SRem(r.ctx, userKey, r.BasicSession.Id()).Err()
}

func (r *Session[U]) Sync(user *U) error {
	userJson, err := json.Marshal(user)
	if err != nil {
		return err
	}
	sids, err := r.Sids((*user).Uid())
	if err != nil {
		return err
	}
	pipe := r.redis.Pipeline()
	for _, sid := range sids {
		pipe.HSet(r.ctx, redisGenKey(r.SidName, sid), r.UserField, userJson)
	}
	_, err = pipe.Exec(r.ctx)
	return err
}

func (r *Session[U]) AllSid(uid uint64) ([]string, error) {
	return r.redis.SMembers(r.ctx, redisGenUserKey(r.KeyPrefix, uid)).Result()
}

func (r *Session[U]) FilterSids(uid uint64, sids []string) ([]string, error) {
	userKey := redisGenUserKey(r.KeyPrefix, uid)
	var filtered []string
	pipe := r.redis.Pipeline()
	for _, sid := range sids {
		if r.redis.Exists(r.ctx, redisGenKey(r.SidName, sid)).Val() > 0 {
			filtered = append(filtered, sid)
		} else {
			pipe.SRem(r.ctx, userKey, sid)
		}
	}
	_, err := pipe.Exec(r.ctx)
	return filtered, err
}

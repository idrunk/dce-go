package session

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"reflect"
	"slices"
	"strconv"
	"time"

	"github.com/idrunk/dce-go/util"
)

const DefaultIdName = "dcesid"
const MinSidLen = 76
const DefaultTtlMinutes = 60

// BasicSession is the core implementation of the IfSession interface.
// It represents a session with a unique session ID (SID), a creation timestamp,
// and a time-to-live (TTL) in minutes. The session can be newly created or loaded
// from an existing session. It supports operations like setting, getting, and
// deleting session fields, as well as renewing the session with new metadata.
// The session can also be cloned, and its data can be serialized or deserialized
// as needed. The BasicSession struct is designed to be extended or embedded in
// other session implementations to provide additional functionality.
type BasicSession struct {
	IfSession
	SidName     string
	ttlMinutes  uint16
	sid         string
	createStamp int64
	touches     bool
	newborn     bool
	sidPool     []string
}

func NewBasicSession(sidPool []string, ttlMinutes uint16) (*BasicSession, error) {
	if len(sidPool) == 0 && ttlMinutes == 0 {
		panic(`"NewWithSid()" was called with an empty "sidPool"`)
	}
	var err error
	var sid string
	var createStamp int64
	newborn := true
	if len(sidPool) > 0 && sidPool[0] != "" {
		newborn = false
		sid = sidPool[0]
		sidPool = slices.Delete(sidPool, 0, 1)
		if ttlMinutes, createStamp, err = parseSid(sid); err != nil {
			return nil, err
		}
	} else if sid, createStamp, err = generateSid(ttlMinutes, &[]string{}); err != nil {
		return nil, err
	}
	return &BasicSession{
		SidName:     DefaultIdName,
		ttlMinutes:  ttlMinutes,
		createStamp: createStamp,
		sid:         sid,
		touches:     false,
		newborn:     newborn,
		sidPool:     sidPool,
	}, nil
}

func parseSid(sid string) (uint16, int64, error) {
	if len(sid) < MinSidLen {
		return 0, 0, util.Closed0(`invalid sid "%s", less then %d chars`, sid, MinSidLen)
	}
	ttlMinutes, err := strconv.ParseUint(sid[64:68], 16, 16)
	if err != nil {
		return 0, 0, err
	}
	stamp, err := strconv.ParseInt(sid[68:], 16, 64)
	if err != nil {
		return 0, 0, err
	}
	return uint16(ttlMinutes), stamp, nil
}

func generateSid(ttlMinutes uint16, sidPool *[]string) (string, int64, error) {
	arrSidPool := *sidPool
	if len(arrSidPool) == 0 || arrSidPool[0] == "" {
		sid, createStamp := GenSid(ttlMinutes)
		return sid, createStamp, nil
	}
	sid := arrSidPool[0]
	*sidPool = slices.Delete(arrSidPool, 0, 1)
	_, stamp, err := parseSid(sid)
	if err != nil {
		return "", 0, err
	}
	return sid, stamp, nil
}

func GenSid(ttlMinutes uint16) (sid string, createStamp int64) {
	now := time.Now()
	hash := sha256.New()
	hash.Write([]byte(fmt.Sprintf("%d-%d", now.UnixNano(), rand.Uint())))
	return fmt.Sprintf("%X%04X%X", hash.Sum(nil), ttlMinutes, now.Unix()), now.Unix()
}

func (b *BasicSession) CloneBasic(cloned IfSession, id string) (*BasicSession, error) {
	basic := *b
	// if passed an id then means with a special id to clone, or else directly clone with original id
	if id != "" {
		if err := basic.ReMeta(id); err != nil {
			return nil, err
		}
	}
	basic.IfSession = cloned
	return &basic, nil
}

func (b *BasicSession) Id() string {
	return b.sid
}

func (b *BasicSession) Key() string {
	return b.Id()
}

func (b *BasicSession) CreateStamp() int64 {
	return b.createStamp
}

func (b *BasicSession) TtlSeconds() uint32 {
	return uint32(b.ttlMinutes) * 60
}

func (b *BasicSession) Newborn() bool {
	return b.newborn
}

func (b *BasicSession) NeedSerial() bool {
	return true
}

func (b *BasicSession) Set(field string, value any) error {
	val, err := TryMarshal(value, b.IfSession.NeedSerial())
	if err != nil {
		return err
	} else if err := b.TryTouch(); err != nil {
		return err
	}
	return b.SilentSet(field, val)
}

func (b *BasicSession) Get(field string, target any) error {
	val, err := b.SilentGet(field)
	if err != nil {
		return err
	} else if err = b.TryTouch(); err != nil {
		return err
	}
	return TryUnmarshal(val, target, b.IfSession.NeedSerial())
}

func TryMarshal(val any, needSerial bool) (any, error) {
	if !needSerial {
		return val, nil
	}
	return json.Marshal(val)
}

func TryUnmarshal(val any, target any, needSerial bool) error {
	if !needSerial {
		rv := reflect.ValueOf(target)
		rv.Elem().Set(reflect.ValueOf(val))
		return nil
	}
	var err error
	if v, ok := val.(string); ok {
		err = json.Unmarshal([]byte(v), target)
	} else if v, ok := val.([]byte); ok {
		err = json.Unmarshal(v, target)
	} else {
		return fmt.Errorf("TryUnmarshal: unknown type %T", val)
	}
	return err
}

func (b *BasicSession) Del(field string) error {
	err := b.SilentDel(field)
	if err != nil {
		return err
	}
	return b.TryTouch()
}

func (b *BasicSession) TryTouch() error {
	if !b.touches {
		if err := b.Touch(); err != nil {
			return err
		}
		b.touches = true
	}
	return nil
}

func (b *BasicSession) ReMeta(sid string) error {
	b.touches = false
	if len(sid) > 0 {
		ttl, createStamp, err := parseSid(sid)
		if err != nil {
			return err
		}
		b.ttlMinutes, b.createStamp, b.sid = ttl, createStamp, sid
	} else {
		id, createStamp, err := generateSid(b.ttlMinutes, &b.sidPool)
		if err != nil {
			return err
		}
		b.sid, b.createStamp = id, createStamp
	}
	return nil
}

func (b *BasicSession) Renew(filters map[string]any) error {
	raw, err := b.Raw()
	if err != nil {
		return err
	}
	for k, v := range filters {
		if v == nil {
			delete(raw, k)
		} else {
			if val, err := TryMarshal(v, b.IfSession.NeedSerial()); err == nil {
				raw[k] = val
			}
		}
	}
	if err = b.ReMeta(""); err != nil {
		return err
	} else if len(raw) == 0 {
		return nil
	} else if err = b.Load(raw); err != nil {
		return err
	}
	return b.TryTouch()
}

func (b *BasicSession) ListBySids(sids []string) ([]any, error) {
	var sessions []any
	for _, sid := range sids {
		if cl, err := b.Clone(sid); err == nil {
			sessions = append(sessions, cl)
		}
	}
	return sessions, nil
}

type IfSession interface {
	// Id returns the session ID (SID) associated with the session.
	Id() string

	// Key returns the key used to identify the session, which is typically the session ID.
	Key() string

	// CreateStamp returns the timestamp (in Unix time) when the session was created.
	CreateStamp() int64

	// TtlSeconds returns the time-to-live (TTL) for the session in seconds.
	TtlSeconds() uint32

	// Newborn returns a boolean indicating whether the session is newly created (true) or was loaded from an existing session (false).
	Newborn() bool

	// NeedSerial returns a boolean indicating whether the session data needs to be serialized (e.g., JSON) when stored or retrieved.
	NeedSerial() bool

	// Set assigns a value to a specific field in the session. It also handles serialization if needed and triggers a touch event.
	Set(field string, value any) error

	// Get retrieves the value of a specific field from the session and assigns it to the target. It also triggers a touch event.
	Get(field string, target any) error

	// Del removes a specific field from the session. It also triggers a touch event.
	Del(field string) error

	// SilentSet assigns a value to a specific field in the session without triggering a touch event.
	SilentSet(field string, value any) error

	// SilentGet retrieves the value of a specific field from the session without triggering a touch event.
	SilentGet(field string) (any, error)

	// SilentDel removes a specific field from the session without triggering a touch event.
	SilentDel(field string) error

	// Destroy removes the session and all its associated data.
	Destroy() error

	// Touch updates the session's last accessed timestamp, effectively extending its TTL.
	Touch() error

	// TryTouch attempts to update the session's last accessed timestamp if it hasn't been touched recently.
	TryTouch() error

	// Load populates the session with data from the provided map.
	Load(data map[string]any) error

	// Raw returns the raw session data as a map.
	Raw() (map[string]any, error)

	// TtlPassed returns the amount of time (in seconds) that has passed since the session was last accessed.
	TtlPassed() (uint32, error)

	// Renew updates the session with new data based on the provided filters and resets its metadata (e.g., SID and creation timestamp).
	Renew(filters map[string]any) error

	// Clone creates a new session instance with the same properties as the current session, optionally with a new session ID.
	Clone(id string) (any, error)

	// ListBySids retrieves a list of sessions based on the provided session IDs.
	ListBySids(sids []string) ([]any, error)
}

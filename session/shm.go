package session

import (
	"fmt"
	"github.com/idrunk/dce-go/util"
	"math/rand/v2"
	"slices"
	"strconv"
	"sync"
	"time"
)

// sessionMapping map[string]*shmMeta
var sessionMapping = util.NewStruct[sync.Map]()

// userMapping map[string][]string
var userMapping = util.NewStruct[sync.Map]()

func ShmDumpMapping() {
	sessionMapping.Range(func(sid, meta any) bool {
		fmt.Printf("%s\n%v\n", sid, meta)
		return true
	})
	userMapping.Range(func(uid, sids any) bool {
		fmt.Printf("%s\n%v\n", uid, sids)
		return true
	})
}

type shmMeta struct {
	data        map[string]any
	expireStamp int64
}

type ShmSession[U UidGetter] struct {
	*BasicSession
	*UserSession[U]
	*ConnectionSession
}

func NewShmSession[U UidGetter](sidPool []string, ttlMinutes uint16) (*ShmSession[U], error) {
	basic, err := NewBasicSession(sidPool, ttlMinutes)
	if err != nil {
		return nil, err
	}
	rs := &ShmSession[U]{
		BasicSession:      basic,
		UserSession:       NewUserSession[U](basic),
		ConnectionSession: NewConnectionSession(basic),
	}
	rs.BasicSession.IfSession = rs
	rs.UserSession.IfUserSession = rs
	rs.ConnectionSession.IfConnection = rs
	return rs, nil
}

func (s *ShmSession[U]) meta(autoGen bool) (*shmMeta, error) {
	if m, ok := sessionMapping.Load(s.Key()); ok {
		return m.(*shmMeta), nil
	} else if autoGen {
		sessionMapping.Store(s.Key(), &shmMeta{data: make(map[string]any)})
		return s.meta(autoGen)
	}
	return nil, util.Closed0(`Sid "%s" could not be found id mapping`, s.Key())
}

func (s *ShmSession[U]) NeedSerial() bool {
	return false
}

func (s *ShmSession[U]) SilentSet(field string, value any) error {
	meta, _ := s.meta(true)
	meta.data[field] = value
	return nil
}

func (s *ShmSession[U]) SilentGet(field string) (any, error) {
	if meta, err := s.meta(false); err != nil {
		return nil, err
	} else if v, ok := meta.data[field]; ok {
		return v, nil
	}
	return nil, util.Silent("No session value with key \"%s\"", field)
}

func (s *ShmSession[U]) SilentDel(field string) error {
	if meta, err := s.meta(false); err == nil {
		delete(meta.data, field)
	}
	return nil
}

func (s *ShmSession[U]) Destroy() error {
	sessionMapping.Delete(s.Key())
	return nil
}

func (s *ShmSession[U]) Touch() error {
	meta, err := s.meta(true)
	if err != nil {
		return err
	}
	meta.expireStamp = time.Now().Unix() + int64(s.TtlSeconds())
	s.tryClear()
	return nil
}

func (s *ShmSession[U]) Load(data map[string]any) error {
	meta, _ := s.meta(true)
	meta.data = data
	return nil
}

func (s *ShmSession[U]) Raw() (map[string]any, error) {
	if meta, err := s.meta(false); err == nil {
		return meta.data, nil
	}
	return make(map[string]any), nil
}

func (s *ShmSession[U]) TtlPassed() (uint32, error) {
	meta, err := s.meta(false)
	if err != nil {
		return 0, err
	} else if meta.expireStamp < 1 {
		return 0, util.Closed0("ttl was not initialized yet.")
	}
	return uint32(time.Now().Unix() - meta.expireStamp + int64(s.TtlSeconds())), nil
}

func (s *ShmSession[U]) Renew(filters map[string]any) error {
	err := s.UserSession.Renew(filters)
	if err == nil && s.Request() {
		return s.UpdateShadow(s.BasicSession.Id())
	}
	return err
}

func (s *ShmSession[U]) Clone(id string) (any, error) {
	cloned := *s
	cl := &cloned
	basic, err := s.CloneBasic(cl, id)
	if err != nil {
		return nil, err
	}
	cl.BasicSession = basic
	cl.UserSession = s.CloneUser(cl, basic)
	cl.ConnectionSession = s.CloneConnection(cl, basic)
	return cl, nil
}

var mu sync.Mutex

func (s *ShmSession[U]) tryClear() {
	if !mu.TryLock() || rand.UintN(10) > 2 {
		return
	}
	go func() {
		defer mu.Unlock()
		now := time.Now().Unix()
		sessionMapping.Range(func(sid, meta any) bool {
			m := meta.(*shmMeta)
			if m.expireStamp > 0 && m.expireStamp <= now {
				sessionMapping.Delete(sid)
			}
			return true
		})
	}()
}

func shmGenUserKey(id uint64) string {
	return strconv.FormatUint(id, 10)
}

func (s *ShmSession[U]) filterSids(userKey string) []string {
	sids := make([]string, 0, 1)
	if v, ok := userMapping.LoadOrStore(userKey, make([]string, 1)); ok {
		sids = v.([]string)
		for i := len(sids) - 1; i >= 0; i-- {
			// If the session to witch the sid belongs does not exist, remove it
			if _, ok = sessionMapping.Load(sids[i]); !ok {
				sids = slices.Delete(sids, i, i+1)
			}
		}
	}
	return sids
}

func (s *ShmSession[U]) Mapping() error {
	userKey, err := s.UserKey()
	if err != nil {
		return err
	}
	sids := append(s.filterSids(userKey), s.Id())
	userMapping.Store(userKey, sids)
	return nil
}

func (s *ShmSession[U]) Unmapping() error {
	userKey, err := s.UserKey()
	if err != nil {
		return err
	}
	sids := s.filterSids(userKey)
	if index := slices.Index(sids, s.Id()); index > -1 {
		sids = slices.Delete(sids, index, index+1)
		if len(sids) == 0 {
			userMapping.Delete(userKey)
			return nil
		}
	}
	userMapping.Store(userKey, sids)
	return nil
}

func (s *ShmSession[U]) Sync(user *U) error {
	sids, err := s.Sids((*user).Uid())
	if err != nil {
		return err
	}
	for _, sid := range sids {
		if meta, ok := sessionMapping.Load(sid); ok {
			m := meta.(*shmMeta)
			m.data[s.UserField] = user
		}
	}
	return nil
}

func (s *ShmSession[U]) AllSid(uid uint64) ([]string, error) {
	userKey := shmGenUserKey(uid)
	if v, ok := userMapping.Load(userKey); ok {
		return v.([]string), nil
	}
	return nil, util.Silent("No mapping with key \"%s\"", userKey)
}

func (s *ShmSession[U]) FilterSids(uid uint64, sids []string) ([]string, error) {
	allValid := s.filterSids(shmGenUserKey(uid))
	for i := len(sids) - 1; i >= 0; i-- {
		if !slices.Contains(allValid, sids[i]) {
			sids = slices.Delete(sids, i, i+1)
		}
	}
	return sids, nil
}

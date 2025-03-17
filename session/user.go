package session

import (
	"strconv"

	"go.drunkce.com/dce/util"
)

const DefaultUserPrefix = "dceusmap"

const DefaultUserField = "$user"

const MappingTtlSeconds = 60 * 60 * 24 * 7

const (
	notLoaded int8 = iota - 1
	loadedNone
	loadedSome
)

// UserSession is a generic struct that represents a session associated with a user.
// It embeds a BasicSession and implements the IfUserSession interface, providing
// functionality to manage user sessions, including login, logout, and session renewal.
// The struct is parameterized with a type U that must implement the UidGetter interface,
// which requires a method to retrieve the user's unique identifier (UID).
//
// Fields:
//   - BasicSession: The embedded basic session that provides core session management capabilities.
//   - IfUserSession[U]: The interface that defines user-specific session operations.
//   - KeyPrefix: A string prefix used for generating user-specific keys.
//   - UserField: A string field name used to store user data within the session.
//   - loadState: An int8 value indicating the current state of user data loading.
//   - user: The user data of type U associated with the session.
type UserSession[U UidGetter] struct {
	*BasicSession
	IfUserSession[U]
	KeyPrefix string
	UserField string
	loadState int8
	user      U
}

func NewUserSession[U UidGetter](basic *BasicSession) *UserSession[U] {
	return &UserSession[U]{BasicSession: basic, KeyPrefix: DefaultUserPrefix, UserField: DefaultUserField, loadState: notLoaded}
}

func (s *UserSession[U]) CloneUser(cloned IfUserSession[U], basic *BasicSession) *UserSession[U] {
	user := *s
	user.IfUserSession = cloned
	user.BasicSession = basic
	user.user = util.NewStruct[U]()
	user.loadState = notLoaded
	return &user
}

func (s *UserSession[U]) UserKey() (string, error) {
	uid, err := s.Uid()
	if err != nil {
		return "", err
	}
	return strconv.FormatUint(uid, 10), nil
}

func (s *UserSession[U]) User() (U, bool) {
	if s.loadState == notLoaded {
		if val, err := s.SilentGet(s.UserField); err == nil {
			var user U
			if err = TryUnmarshal(val, &user, s.IfSession.NeedSerial()); err == nil {
				s.user = user
				s.loadState = loadedSome
				return s.user, true
			}
		}
		s.loadState = loadedNone
	}
	return s.user, s.loadState > loadedNone
}

func (s *UserSession[U]) Uid() (uint64, error) {
	if u, ok := s.User(); !ok {
		return 0, util.Closed0("User not loaded, cannot get uid")
	} else {
		return u.Uid(), nil
	}
}

func (s *UserSession[U]) doLogin(user *U, ttlMinutes uint16) error {
	cl, err := s.Clone("")
	if err != nil {
		return err
	}
	cloned := cl.(IfSession)
	if ttlMinutes > 0 {
		s.ttlMinutes = ttlMinutes
	}
	if user != nil {
		s.user = *user
		s.loadState = loadedSome
	}
	filters := make(map[string]any)
	if s.loadState == loadedSome {
		filters[s.UserField] = s.user
	}
	// should call this via the interface, it will locate from the implementation
	// for login just directly regenerate a new sid
	if err := s.IfSession.Renew(filters); err != nil {
		return err
	}
	_ = cloned.Destroy()
	return nil
}

func (s *UserSession[U]) Login(user U, ttlMinutes uint16) error {
	return s.doLogin(&user, ttlMinutes)
}

func (s *UserSession[U]) AutoLogin() error {
	return s.doLogin(nil, 0)
}

func (s *UserSession[U]) Logout() error {
	if s.loadState < loadedSome {
		return nil
	} else if err := s.Unmapping(); err != nil {
		return err
	}
	s.user = util.NewStruct[U]()
	s.loadState = loadedNone
	return s.SilentDel(s.UserField)
}

func (s *UserSession[U]) Sids(uid uint64) ([]string, error) {
	sids, err := s.AllSid(uid)
	if err != nil {
		return nil, err
	}
	return s.FilterSids(uid, sids)
}

func (s *UserSession[U]) Renew(filters map[string]any) error {
	if err := s.BasicSession.Renew(filters); err != nil {
		return err
	}
	s.loadState = notLoaded
	_ = s.Mapping()
	return nil
}

func (s *UserSession[U]) ListByUid(uid uint64) ([]any, error) {
	if sids, err := s.Sids(uid); err != nil {
		return nil, err
	} else {
		return s.ListBySids(sids)
	}
}

type IfUserSession[U UidGetter] interface {
	// Mapping establishes a mapping between the user and the session.
	// It returns an error if the mapping fails.
	Mapping() error

	// Unmapping removes the mapping between the user and the session.
	// It returns an error if the unmapping fails.
	Unmapping() error

	// Sync synchronizes the session with the provided user data.
	// It returns an error if the synchronization fails.
	Sync(user *U) error

	// AllSid retrieves all session IDs (SIDs) associated with the given user ID (UID).
	// It returns a slice of SIDs and an error if the retrieval fails.
	AllSid(uid uint64) ([]string, error)

	// FilterSids filters the provided session IDs (SIDs) based on the given user ID (UID).
	// It returns a filtered slice of SIDs and an error if the filtering fails.
	FilterSids(uid uint64, sids []string) ([]string, error)

	// UserKey generates a unique key for the user based on their UID.
	// It returns the generated key and an error if the key generation fails.
	UserKey() (string, error)

	// User retrieves the user associated with the session.
	// It returns the user and a boolean indicating whether the user was successfully loaded.
	User() (U, bool)

	// Uid retrieves the UID of the user associated with the session.
	// It returns the UID and an error if the UID retrieval fails.
	Uid() (uint64, error)

	// doLogin performs the login operation for the session, optionally with a provided user and TTL (Time-To-Live) in minutes.
	// It returns an error if the login operation fails.
	doLogin(user *U, ttlMinutes uint16) error

	// Login logs in the provided user with the specified TTL (Time-To-Live) in minutes.
	// It returns an error if the login operation fails.
	Login(user U, ttlMinutes uint16) error

	// AutoLogin attempts to automatically log in the user without providing explicit user data.
	// It returns an error if the auto-login operation fails.
	AutoLogin() error

	// Logout logs out the user from the session.
	// It returns an error if the logout operation fails.
	Logout() error

	// Sids retrieves all session IDs (SIDs) associated with the given user ID (UID).
	// It returns a slice of SIDs and an error if the retrieval fails.
	Sids(uid uint64) ([]string, error)

	// ListByUid retrieves a list of session data associated with the given user ID (UID).
	// It returns a slice of session data and an error if the retrieval fails.
	ListByUid(uid uint64) ([]any, error)
}

type UidGetter interface {
	Uid() uint64
}

type SimpleUser struct {
	Id     uint64 `json:"id"`
	RoleId uint16 `json:"role_id,omitempty"`
	Nick   string `json:"nick"`
}

func (s SimpleUser) Uid() uint64 {
	return s.Id
}

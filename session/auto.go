package session

import (
	"math"
	"time"

	"github.com/idrunk/dce-go/util"
)

const DefaultNewSidField = "$newid"
const DefaultRenewIntervalSeconds uint16 = 600
const DefaultOriginalJudgmentSeconds uint16 = 120
const DefaultClonedInactiveJudgmentSeconds uint16 = 60

// AutoRenew is a generic struct that provides automatic renewal functionality for sessions.
// It is parameterized with a type S that must implement the IfSession interface.
// The struct contains fields to manage session renewal, including the session itself,
// a field name for storing new session IDs, and various timing configurations for
// renewal and judgment intervals.
type AutoRenew[S IfSession] struct {
	S                             S
	newSidField                   string
	renewIntervalSeconds          uint16
	originalJudgmentSeconds       uint16
	clonedInactiveJudgmentSeconds uint16
}

func NewAutoRenew[S IfSession](s S) *AutoRenew[S] {
	return &AutoRenew[S]{
		S:                             s,
		newSidField:                   DefaultNewSidField,
		renewIntervalSeconds:          DefaultRenewIntervalSeconds,
		originalJudgmentSeconds:       DefaultOriginalJudgmentSeconds,
		clonedInactiveJudgmentSeconds: DefaultClonedInactiveJudgmentSeconds,
	}
}

func (a *AutoRenew[S]) Config(renewIntervalSeconds uint16, originalJudgmentSeconds uint16, clonedInactiveJudgmentSeconds uint16) *AutoRenew[S] {
	if renewIntervalSeconds > 0 {
		a.renewIntervalSeconds = renewIntervalSeconds
	}
	if originalJudgmentSeconds > 0 {
		a.originalJudgmentSeconds = originalJudgmentSeconds
	}
	if clonedInactiveJudgmentSeconds > 0 {
		a.clonedInactiveJudgmentSeconds = clonedInactiveJudgmentSeconds
	}
	return a
}

func (a *AutoRenew[S]) doClone(filters map[string]any) error {
	cl, err := a.S.Clone("")
	if err != nil {
		return err
	} else if err = a.S.Renew(filters); err != nil {
		return err
	}
	cloned := cl.(S)
	_ = cloned.Touch()
	return cloned.SilentSet(a.newSidField, a.S.Id())
}

// TryRenew attempts to renew the session if necessary. It returns a boolean indicating whether the session was renewed
// and an error if any occurred during the renewal process.
//
// The function first checks if the session is a newborn ancestor. If it is, the function immediately returns true,
// indicating that no renewal is needed.
//
// If the session is not a newborn, the function calculates the time elapsed since the session was created minus the
// renewal interval. If this value is negative, it means the session has not yet expired, and the function attempts
// to touch the session (update its last accessed time) and returns false, indicating no renewal was performed.
//
// If the session has exceeded the renewal interval, the function checks if a new session ID (newSid) is stored in the
// session. If a newSid exists and the session has exceeded the original judgment interval, the function clones the
// session using the newSid. It then compares the TTL (Time To Live) of the cloned session with the original session.
// If the cloned session's TTL is less than the cloned inactive judgment interval and also less than the original
// session's TTL, the original session is destroyed, and an error is returned indicating that the session was destroyed.
//
// If the cloned session's TTL is not less than the cloned inactive judgment interval, the cloned session is destroyed,
// and the original session is renewed by cloning it again with the newSid field cleared. The function then returns true,
// indicating that the session was renewed.
//
// If no newSid is found, the function attempts to clone the session without any filters and returns true if successful.
func (a *AutoRenew[S]) TryRenew() (bool, error) {
	if a.S.Newborn() {
		// directly return true if is a newborn ancestor
		return true, nil
	}
	secondFromRenew := time.Now().Unix() - a.S.CreateStamp() - int64(a.renewIntervalSeconds)
	if secondFromRenew < 0 {
		// directly touch if not expired
		_ = a.S.TryTouch()
		return false, nil
	} else if newSid, e := a.S.SilentGet(a.newSidField); e == nil {
		if secondFromRenew > int64(a.originalJudgmentSeconds) {
			cl, err := a.S.Clone(newSid.(string))
			if err != nil {
				return false, err
			}
			cloned := cl.(S)
			newTp, err := cloned.TtlPassed()
			if err != nil {
				newTp = math.MaxUint32
			}
			if newTp < uint32(a.clonedInactiveJudgmentSeconds) {
				if oldTp, err := a.S.TtlPassed(); err == nil && newTp < oldTp {
					if err := a.S.Destroy(); err != nil {
						return false, err
					}
					return false, util.Closed0(`session "%s" was destroyed, unable to continue use`, a.S.Id())
				}
			}
			if err = cloned.Destroy(); err != nil {
				return false, err
			} else if err = a.doClone(map[string]any{a.newSidField: nil}); err != nil {
				return false, err
			}
			return true, nil
		}
		_ = a.S.TryTouch()
		return false, nil
	} else if e = a.doClone(map[string]any{}); e != nil {
		return false, e
	}
	return true, nil
}

package router

import (
	"bytes"
	"context"
	"sync"
	"time"

	"go.drunkce.com/dce/session"
	"go.drunkce.com/dce/util"
)

const (
	ContextKeyRespSid  = "Resp-Session-Id"
	HttpContentTypeKey = "Content-Type"
	sessionKey         = "$#session#"
)

// Meta is a generic struct that encapsulates metadata and state associated with a request.
// It provides methods for managing the request context, session data, response buffer,
// and error handling. The struct is parameterized by the type of the request (Req),
// allowing it to be used with different request types while maintaining type safety.
//
// Fields:
//   - Req: The request object of type Req, which holds the actual request data.
//   - respBuffer: A bytes.Buffer used to accumulate the response data before it is sent.
//   - err: An error that can be set during request processing to indicate a failure.
//   - ctxData: A map[string]any that stores arbitrary context data associated with the request.
//   - context: A context.Context that manages the lifecycle and cancellation of the request.
//   - mu: A pointer to a sync.RWMutex used to synchronize access to the struct's fields.
type Meta[Req any] struct {
	Req        Req
	respBuffer bytes.Buffer
	err        error
	ctxData    map[string]any
	context    context.Context
	mu         *sync.RWMutex
}

func NewMeta[Req any](req Req, ctxData map[string]any, initContext bool) Meta[Req] {
	if ctxData == nil {
		ctxData = make(map[string]any)
	}
	m := Meta[Req]{Req: req, ctxData: ctxData, mu: &sync.RWMutex{}}
	if initContext {
		m.context = context.Background()
	}
	return m
}

func (m *Meta[Req]) ClearBuffer() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	bs := m.respBuffer.Bytes()
	m.respBuffer.Reset()
	return bs
}

func (m *Meta[Req]) ResponseEmpty() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.respBuffer.Len() == 0
}

func (m *Meta[Req]) TryPrintErr() {
	if m.err != nil {
		println(m.err.Error())
	}
}

func (m *Meta[Req]) Id() uint32 {
	return 0
}

func (m *Meta[Req]) Sid() string {
	return ""
}

func (m *Meta[Req]) MatchApi([]*Api) (index int) {
	return 0
}

func (m *Meta[Req]) Write(bytes []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.respBuffer.Write(bytes)
}

func (m *Meta[Req]) WriteString(str string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.respBuffer.WriteString(str)
}

func (m *Meta[Req]) SetError(err error) {
	m.err = err
}

func (m *Meta[Req]) Error() error {
	return m.err
}

func (m *Meta[Req]) ErrorUnits() (int, string) {
	if err := m.Error(); err != nil {
		return util.ResponseUnits(err)
	} else {
		return 0, ""
	}
}

func (m *Meta[Req]) SetCtxData(key string, val any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ctxData[key] = val
}

func (m *Meta[Req]) CtxData(key string) (any, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if v, ok := m.ctxData[key]; ok {
		return v, true
	}
	return nil, false
}

func (m *Meta[Req]) SetSession(session session.IfSession) {
	m.SetCtxData(sessionKey, session)
}

func (m *Meta[Req]) Session() session.IfSession {
	if v, ok := m.CtxData(sessionKey); ok {
		return v.(session.IfSession)
	}
	return nil
}

func (m *Meta[Req]) SetRespSid(sid string) {
	m.SetCtxData(ContextKeyRespSid, sid)
}

func (m *Meta[Req]) RespSid() string {
	if v, ok := m.CtxData(ContextKeyRespSid); ok {
		return v.(string)
	}
	return ""
}

func (m *Meta[Req]) Deadline() (deadline time.Time, ok bool) {
	return m.context.Deadline()
}

func (m *Meta[Req]) Done() <-chan struct{} {
	return m.context.Done()
}

func (m *Meta[Req]) Err() error {
	return m.context.Err()
}

func (m *Meta[Req]) Value(key any) any {
	return m.context.Value(key)
}

// RoutableProtocol defines the interface for a protocol that can be routed within the application.
// It provides methods for handling requests, managing session data, and writing responses.
// Implementations of this interface are expected to provide functionality for identifying
// the request, matching it to an API, managing errors, and interacting with the context.
type RoutableProtocol interface {
	// Id returns a unique identifier for the request. This is typically used to distinguish
	// between different requests in a system.
	Id() uint32
 
	// Path returns the path associated with the request. This is typically the URL path
	// or a similar identifier that specifies the resource being accessed.
	Path() string

	// MatchApi matches the request against a list of APIs and returns the index of the
	// matching API. If no match is found, it returns -1.
	MatchApi(apis []*Api) (index int)

	// Body retrieves the body of the request as a byte slice. It returns an error if
	// the body cannot be read or processed.
	Body() ([]byte, error)

	// Sid returns the session ID associated with the request. This is typically used
	// to identify the user session.
	Sid() string

	// Write writes the provided byte slice to the response buffer. It returns the number
	// of bytes written and any error encountered.
	Write(bytes []byte) (int, error)

	// WriteString writes the provided string to the response buffer. It returns the number
	// of bytes written and any error encountered.
	WriteString(str string) (int, error)

	// SetError sets an error on the request. This is typically used to propagate errors
	// that occur during request processing.
	SetError(error)

	// Error returns the error associated with the request, if any.
	Error() error

	// SetCtxData sets a key-value pair in the context data map. This is used to store
	// arbitrary data associated with the request.
	SetCtxData(key string, val any)

	// CtxData retrieves the value associated with the given key from the context data map.
	// It returns the value and a boolean indicating whether the key was found.
	CtxData(key string) (any, bool)

	// SetSession sets the session associated with the request. This is typically used
	// to manage user sessions.
	SetSession(session session.IfSession)

	// Session retrieves the session associated with the request. It returns nil if no
	// session is set.
	Session() session.IfSession

	// SetRespSid sets the session ID in the response context. This is typically used
	// to propagate session information to the response.
	SetRespSid(sid string)

	// RespSid retrieves the session ID from the response context. It returns an empty
	// string if no session ID is set.
	RespSid() string

	// Deadline returns the time when the request context will be canceled, if any.
	// It also returns a boolean indicating whether a deadline is set.
	Deadline() (deadline time.Time, ok bool)

	// Done returns a channel that is closed when the request context is canceled.
	// This can be used to detect when the request should be terminated.
	Done() <-chan struct{}

	// Err returns the error that caused the request context to be canceled, if any.
	Err() error

	// Value retrieves the value associated with the given key from the request context.
	// It returns nil if the key is not found.
	Value(key any) any
}

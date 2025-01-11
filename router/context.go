package router

import (
	"strings"
	"time"

	"github.com/idrunk/dce-go/util"
)

// Context is a generic struct that encapsulates the state and functionality for handling RoutableProtocol requests.
// It is parameterized by the type Rp, which must implement the RoutableProtocol interface.
// The Context struct holds references to the request protocol (Rp), the API (Api), the router (router),
// a suffix (suffix) for path matching, and a map of path parameters (params).
// It provides methods to set routes, retrieve path parameters, handle request bodies, write responses,
// and manage request context such as deadlines, cancellation, and error handling.
type Context[Rp RoutableProtocol] struct {
	Rp     Rp
	Api    *RpApi[Rp]
	router *Router[Rp]
	suffix util.Tuple2[*Suffix, bool]
	params map[string]Param
}

func (c *Context[Rp]) SetRoutes(router *Router[Rp], api *RpApi[Rp], pathParams map[string]Param, suffix *Suffix) {
	c.Api = api
	c.suffix = util.NewTuple2(suffix, false)
	c.params = pathParams
	c.router = router
}

func (c *Context[Rp]) Suffix() *Suffix {
	if c.suffix.B {
		return c.suffix.A
	} else if c.suffix.A == nil {
		if suffix, ok := util.SeqFrom(c.Api.Suffixes).Find(func(s Suffix) bool {
			return strings.HasSuffix(c.Rp.Path(), string(s))
		}); ok {
			c.suffix.A = &suffix
		}
	}
	c.suffix.B = true
	return c.suffix.A
}

func (c *Context[Rp]) Param(key string) string {
	if param, ok := c.params[key]; ok {
		return param.Value()
	}
	return ""
}

func (c *Context[Rp]) Params(key string) []string {
	if param, ok := c.params[key]; ok {
		return param.Values()
	}
	return []string{}
}

func (c *Context[Rp]) Body() ([]byte, error) {
	return c.Rp.Body()
}

func (c *Context[Rp]) Write(bytes []byte) (int, error) {
	return c.Rp.Write(bytes)
}

func (c *Context[Rp]) WriteString(str string) (int, error) {
	return c.Rp.WriteString(str)
}

func (c *Context[Rp]) SetError(err error) bool {
	c.Rp.SetError(err)
	return true
}

func (c *Context[Rp]) Deadline() (deadline time.Time, ok bool) {
	return c.Rp.Deadline()
}

func (c *Context[Rp]) Done() <-chan struct{} {
	return c.Rp.Done()
}

func (c *Context[Rp]) Err() error {
	return c.Rp.Err()
}

func (c *Context[Rp]) Value(key any) any {
	return c.Rp.Value(key)
}

func NewContext[Rp RoutableProtocol](rp Rp) *Context[Rp] {
	return &Context[Rp]{Rp: rp}
}

type Param struct {
	string
	vec  []string
	Type int
}

func NewParam(val any, ty int) Param {
	param := Param{Type: ty}
	if param.Type&(VarTypeEmptableVector|VarTypeVector) > 0 {
		param.vec = val.([]string)
	} else {
		param.string = val.(string)
	}
	return param
}

func (p *Param) Value() string {
	return p.string
}

func (p *Param) Values() []string {
	return p.vec
}

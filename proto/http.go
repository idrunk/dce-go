package proto

import (
	"errors"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.drunkce.com/dce/router"
	"go.drunkce.com/dce/util"
)

var (
	HttpGet     = router.Method(1)
	HttpPost    = router.Method(2)
	HttpPut     = router.Method(3)
	HttpDelete  = router.Method(4)
	HttpHead    = router.Method(5)
	HttpOptions = router.Method(6)
	HttpConnect = router.Method(7)
	HttpPatch   = router.Method(8)
	HttpTrace   = router.Method(9)
)

type Http = router.Context[*HttpProtocol]

const HeaderSidKey = "X-Session-Id"

type HttpProtocol struct {
	router.Meta[*http.Request]
	Writer http.ResponseWriter
}

func (h *HttpProtocol) Path() string {
	return h.Req.URL.Path[1:]
}

func (h *HttpProtocol) MatchApi(apis []*router.Api) (index int) {
	return slices.IndexFunc(apis, func(api *router.Api) bool {
		if method := uint(ToUintMethod(h.Req.Method)); method < 1 || method&uint(api.Method) != method {
			return false
		} else if hosts := api.Hosts(); len(hosts) > 0 {
			for _, host := range hosts {
				if strings.Contains(host, ":") {
					if host == h.Req.Host {
						return true
					}
				} else if _, err := strconv.Atoi(host); err == nil {
					if strings.HasSuffix(h.Req.Host, ":"+host) {
						return true
					}
				} else if strings.HasPrefix(h.Req.Host, host+":") {
					return true
				}
			}
			return false
		}
		return true
	})
}

func (h *HttpProtocol) Body() ([]byte, error) {
	return io.ReadAll(h.Req.Body)
}

var headerSidKey string = strings.ToLower(HeaderSidKey)

func (h *HttpProtocol) Sid() string {
	if headerSid := h.Req.Header.Get(HeaderSidKey); len(headerSid) > 0 {
		return headerSid
	} else if cookies := h.Req.Cookies(); len(cookies) > 0 {
		if cookie, ok := util.SeqFrom(cookies).Find(func(c *http.Cookie) bool {
			lower := strings.ToLower((*c).Name)
			return lower == "session_id" || lower == "session-id" || lower == headerSidKey
		}); ok {
			return (*cookie).Value
		}
	}
	return ""
}

func (h *HttpProtocol) Deadline() (deadline time.Time, ok bool) {
	return h.Req.Context().Deadline()
}

func (h *HttpProtocol) Done() <-chan struct{} {
	return h.Req.Context().Done()
}

func (h *HttpProtocol) Err() error {
	return h.Req.Context().Err()
}

func (h *HttpProtocol) Value(key any) any {
	return h.Req.Context().Value(key)
}

var methodNameUintMapping = map[string]router.Method{
	"GET":     HttpGet,
	"POST":    HttpPost,
	"PUT":     HttpPut,
	"DELETE":  HttpDelete,
	"HEAD":    HttpHead,
	"OPTIONS": HttpOptions,
	"CONNECT": HttpConnect,
	"PATCH":   HttpPatch,
	"TRACE":   HttpTrace,
}

func ToUintMethod(name string) router.Method {
	if methodUint, ok := methodNameUintMapping[name]; ok {
		return methodUint
	}
	return 0
}

type WrappedHttpRouter router.Router[*HttpProtocol]

func (h *WrappedHttpRouter) Raw() *router.Router[*HttpProtocol] {
	return (*router.Router[*HttpProtocol])(h)
}

func (h *WrappedHttpRouter) Get(path string, controller func(h *Http)) *WrappedHttpRouter {
	return h.pushMethod(HttpGet|HttpHead, path, controller)
}

func (h *WrappedHttpRouter) Post(path string, controller func(h *Http)) *WrappedHttpRouter {
	return h.pushMethod(HttpPost|HttpOptions, path, controller)
}

func (h *WrappedHttpRouter) Put(path string, controller func(h *Http)) *WrappedHttpRouter {
	return h.pushMethod(HttpPut|HttpOptions, path, controller)
}

func (h *WrappedHttpRouter) Patch(path string, controller func(h *Http)) *WrappedHttpRouter {
	return h.pushMethod(HttpPatch|HttpOptions, path, controller)
}

func (h *WrappedHttpRouter) Delete(path string, controller func(h *Http)) *WrappedHttpRouter {
	return h.pushMethod(HttpDelete|HttpOptions, path, controller)
}

func (h *WrappedHttpRouter) pushMethod(method router.Method, path string, controller func(h *Http)) *WrappedHttpRouter {
	return h.PushApi(router.Api{Method: method, Path: path}, controller)
}

func (h *WrappedHttpRouter) PushApi(api router.Api, controller func(c *Http)) *WrappedHttpRouter {
	if api.Method == 0 {
		panic(`Please specify the http "method" property`)
	}
	h.Raw().PushApi(api, controller)
	return h
}

func (h *WrappedHttpRouter) Route(writer http.ResponseWriter, request *http.Request) {
	hp := NewHttpProtocol(writer, request)
	context := router.NewContext(hp)
	h.Raw().Route(context)
	hp.TryPrintErr()
	if ct, ok := hp.CtxData(router.HttpContentTypeKey); ok {
		hp.Writer.Header().Set(router.HttpContentTypeKey, ct.(string))
	}
	if hp.Error() != nil {
		var e util.Error
		if !errors.As(hp.Error(), &e) {
			hp.Writer.WriteHeader(util.ServiceUnavailable)
		} else if ! e.IsOpenly() || hp.ResponseEmpty() {
			hp.Writer.WriteHeader(util.Iif(e.Code > 0 && e.Code < 600, e.Code, util.ServiceUnavailable))
		}
	}
	if sid := hp.RespSid(); sid != "" {
		hp.Writer.Header().Set(HeaderSidKey, sid)
	}
	bytes := hp.ClearBuffer()
	if _, err := hp.Writer.Write(bytes); err != nil {
		println(err.Error())
	}
}

func NewHttpProtocol(writer http.ResponseWriter, request *http.Request) *HttpProtocol {
	return &HttpProtocol{
		Meta:   router.NewMeta(request, nil, false),
		Writer: writer,
	}
}

var HttpRouter *WrappedHttpRouter

func init() {
	HttpRouter = (*WrappedHttpRouter)(router.ProtoRouter[*HttpProtocol]("http"))
}

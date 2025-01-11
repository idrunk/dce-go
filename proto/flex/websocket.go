package flex

import (
	"bufio"
	"github.com/coder/websocket"
	"github.com/idrunk/dce-go/proto"
	"github.com/idrunk/dce-go/router"
	"log/slog"
	"net/http"
)

type Websocket = router.Context[*WebsocketProtocol]

type WebsocketProtocol struct {
	*PackageProtocol[*http.Request]
}

type WrappedWebsocketRouter struct {
	proto.ConnectorMappingManager[*WebsocketProtocol, *websocket.Conn]
}

func (w *WrappedWebsocketRouter) Route(conn *websocket.Conn, req *http.Request, ctxData map[string]any) bool {
	meta := router.NewMeta(req, ctxData, true)
	ty, reader, err := conn.Reader(&meta)
	if err != nil {
		return w.Except(req.RemoteAddr, err)
	}
	pkg, err := NewPackageProtocolWithMeta(bufio.NewReader(reader), meta)
	if err != nil {
		return w.Except(req.RemoteAddr, err)
	}
	sw := &WebsocketProtocol{pkg}
	context := router.NewContext(sw)
	w.Router.Route(context)
	sw.TryPrintErr()
	if context.Api != nil && context.Api.Responsive {
		bytes := sw.ClearBuffer()
		if err = conn.Write(context, ty, bytes); err != nil {
			slog.Error(err.Error())
		}
	}
	return true
}

var WebsocketRouter *WrappedWebsocketRouter

func init() {
	WebsocketRouter = &WrappedWebsocketRouter{proto.NewConnectorMappingManager[*WebsocketProtocol, *websocket.Conn]("flex-websocket")}
}

package pb

import (
	"log/slog"
	"net"

	"go.drunkce.com/dce/proto"
	"go.drunkce.com/dce/proto/flex"
	"go.drunkce.com/dce/router"
)

type Tcp = router.Context[*TcpProtocol]

type TcpProtocol struct {
	*PackageProtocol[net.Conn]
}

type WrappedTcpRouter struct {
	proto.ConnectorMappingManager[*TcpProtocol, net.Conn]
}

func (t *WrappedTcpRouter) Route(conn net.Conn, ctxData map[string]any) bool {
	data, err := flex.StreamRead(conn)
	if err != nil {
		// Reading failure is considered as connection loss, just return false to let loop break
		return t.Except(conn.RemoteAddr().String(), err)
	}
	pkg, err := NewPackageProtocol(data, router.NewMeta(conn, ctxData, true))
	if err != nil {
		return t.Warn(err)
	}
	sw := &TcpProtocol{pkg}
	context := router.NewContext(sw)
	t.Router.Route(context)
	sw.TryPrintErr()
	if context.Api != nil && context.Api.Responsive {
		bytes := sw.ClearBuffer()
		if _, err = conn.Write(flex.StreamPack(bytes)); err != nil {
			slog.Error(err.Error())
		}
	}
	return true
}

var TcpRouter *WrappedTcpRouter

func init() {
	TcpRouter = &WrappedTcpRouter{proto.NewConnectorMappingManager[*TcpProtocol, net.Conn]("pb-tcp")}
}

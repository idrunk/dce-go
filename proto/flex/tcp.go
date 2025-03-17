package flex

import (
	"bufio"
	"log/slog"
	"net"

	"go.drunkce.com/dce/proto"
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
	pkg, err := NewPackageProtocol(bufio.NewReader(conn), conn, ctxData)
	if err != nil {
		return t.Except(conn.RemoteAddr().String(), err)
	}
	sw := &TcpProtocol{pkg}
	context := router.NewContext(sw)
	t.Router.Route(context)
	sw.TryPrintErr()
	if context.Api != nil && context.Api.Responsive {
		bytes := sw.ClearBuffer()
		if _, err = conn.Write(bytes); err != nil {
			slog.Error(err.Error())
		}
	}
	return true
}

var TcpRouter *WrappedTcpRouter

func init() {
	TcpRouter = &WrappedTcpRouter{proto.NewConnectorMappingManager[*TcpProtocol, net.Conn]("flex-tcp")}
}

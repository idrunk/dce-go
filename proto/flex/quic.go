package flex

import (
	"bufio"
	"github.com/idrunk/dce-go/proto"
	"github.com/idrunk/dce-go/router"
	"github.com/quic-go/quic-go"
)

type Quic = router.Context[*QuicProtocol]

type QuicProtocol struct {
	*PackageProtocol[quic.Connection]
}

type WrappedQuicRouter struct {
	proto.ConnectorMappingManager[*QuicProtocol, quic.Connection]
}

func (q *WrappedQuicRouter) Route(conn quic.Connection, ctxData map[string]any) bool {
	meta := router.NewMeta(conn, ctxData, true)
	stream, err := conn.AcceptStream(&meta)
	if err != nil {
		return q.Except(conn.RemoteAddr().String(), err)
	}
	defer stream.Close()
	context, qp, ok := q.uniRoute(stream, meta)
	if !ok {
		return false
	}
	if context.Api != nil && context.Api.Responsive {
		bts := qp.ClearBuffer()
		if _, err = stream.Write(bts); err != nil {
			println(err.Error())
		}
	}
	return true
}

func (q *WrappedQuicRouter) UniRoute(conn quic.Connection, ctxData map[string]any) bool {
	meta := router.NewMeta(conn, ctxData, true)
	stream, err := conn.AcceptUniStream(&meta)
	if err != nil {
		return q.Except(conn.RemoteAddr().String(), err)
	}
	_, _, ok := q.uniRoute(stream, meta)
	return ok
}

func (q *WrappedQuicRouter) uniRoute(stream quic.ReceiveStream, meta router.Meta[quic.Connection]) (*Quic, *QuicProtocol, bool) {
	pkgProto, err := NewPackageProtocolWithMeta(bufio.NewReader(stream), meta)
	if err != nil {
		q.Except(meta.Req.RemoteAddr().String(), err)
		return nil, nil, false
	}
	qp := &QuicProtocol{pkgProto}
	context := router.NewContext(qp)
	q.Router.Route(context)
	qp.TryPrintErr()
	return context, qp, true
}

var QuicRouter *WrappedQuicRouter

func init() {
	QuicRouter = &WrappedQuicRouter{proto.NewConnectorMappingManager[*QuicProtocol, quic.Connection]("flex-quic")}
}

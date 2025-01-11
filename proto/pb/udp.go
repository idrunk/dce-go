package pb

import (
	"fmt"
	"github.com/idrunk/dce-go/router"
	"net"
)

type Udp = router.Context[*UdpProtocol]

type UdpProtocol struct {
	*PackageProtocol[*net.UDPAddr]
}

func UdpRoute(conn *net.UDPConn, pkg []byte, addr *net.UDPAddr, ctxData map[string]any) {
	pkgProto, err := NewPackageProtocol(pkg, router.NewMeta(addr, ctxData, true))
	if err != nil {
		println(fmt.Sprintf("Package parse failed with: %s", err.Error()))
		return
	}
	sw := &UdpProtocol{pkgProto}
	context := router.NewContext(sw)
	UdpRouter.Route(context)
	sw.TryPrintErr()
	if context.Api != nil && context.Api.Responsive {
		bts := sw.ClearBuffer()
		if _, err = conn.WriteToUDP(bts, addr); err != nil {
			println(err.Error())
		}
	}
}

var UdpRouter *router.Router[*UdpProtocol]

func init() {
	UdpRouter = router.ProtoRouter[*UdpProtocol]("pb-udp")
}

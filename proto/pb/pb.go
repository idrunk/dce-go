package pb

import (
	"github.com/idrunk/dce-go/router"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"math"
	"sync/atomic"
)

type PackageProtocol[Req any] struct {
	router.Meta[Req]
	pkg *Package
}

func (p *PackageProtocol[Req]) Id() uint32 {
	return p.pkg.GetId()
}

func (p *PackageProtocol[Req]) Path() string {
	return p.pkg.GetPath()
}

func (p *PackageProtocol[Req]) Sid() string {
	return p.pkg.GetSid()
}

func (p *PackageProtocol[Req]) Body() ([]byte, error) {
	return p.pkg.GetBody(), nil
}

func (p *PackageProtocol[Req]) ClearBuffer() []byte {
	respSid := p.RespSid()
	p.pkg.Sid = &respSid
	p.pkg.Body = p.Meta.ClearBuffer()
	code, message := p.ErrorUnits()
	i32Code := int32(code)
	p.pkg.Code, p.pkg.Msg = &i32Code, &message
	return pkgSerialize(p.pkg)
}

func NewPackageProtocol[Req any](bts []byte, meta router.Meta[Req]) (*PackageProtocol[Req], error) {
	pkg, err := PackageDeserialize(bts)
	if err != nil {
		return nil, err
	}
	return &PackageProtocol[Req]{meta, pkg}, nil
}

func pkgSerialize(pkg *Package) []byte {
	seq, err := proto.Marshal(pkg)
	if err != nil {
		slog.Warn("Protobuf serialize failed.")
	}
	return seq
}

var reqId atomic.Uint32

func PackageSerialize(path string, body []byte, sid string, id int) []byte {
	if id == -1 {
		reqId.Add(1)
		if id = int(reqId.Load()); id == math.MaxUint32 {
			reqId.Store(0)
		}
	}
	rid := uint32(id)
	return pkgSerialize(&Package{
		Id:   &rid,
		Path: &path,
		Sid:  &sid,
		Code: nil,
		Msg:  nil,
		Body: body,
	})
}

func PackageDeserialize(bts []byte) (*Package, error) {
	var pkg Package
	if err := proto.Unmarshal(bts, &pkg); err != nil {
		return nil, err
	}
	return &pkg, nil
}

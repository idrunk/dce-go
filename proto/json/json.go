package json

import (
	"encoding/json"
	"github.com/idrunk/dce-go/router"
	"math"
	"sync/atomic"
)

type PackageProtocol[Req any] struct {
	router.Meta[Req]
	pkg *Package
}

func (p *PackageProtocol[Req]) Id() uint32 {
	return p.pkg.Id
}

func (p *PackageProtocol[Req]) Path() string {
	return p.pkg.Path
}

func (p *PackageProtocol[Req]) Sid() string {
	return p.pkg.Sid
}

func (p *PackageProtocol[Req]) Body() ([]byte, error) {
	return p.pkg.Body, nil
}

func (p *PackageProtocol[Req]) ClearBuffer() []byte {
	p.pkg.Sid = p.RespSid()
	p.pkg.Body = p.Meta.ClearBuffer()
	code, message := p.ErrorUnits()
	p.pkg.Code, p.pkg.Msg = int32(code), message
	return p.pkg.Serialize()
}

func NewPackageProtocol[Req any](data []byte, meta router.Meta[Req]) (*PackageProtocol[Req], error) {
	pkg, err := PackageDeserialize(data)
	if err != nil {
		return nil, err
	}
	return &PackageProtocol[Req]{meta, pkg}, nil
}

type Package struct {
	Id   uint32 `json:"id,omitempty"`
	Path string `json:"path,omitempty"`
	Sid  string `json:"sid,omitempty"`
	Code int32  `json:"code,omitempty"`
	Msg  string `json:"msg,omitempty"`
	Body []byte `json:"body,omitempty"`
}

func (p *Package) Serialize() []byte {
	if seq, err := json.Marshal(p); err == nil {
		return seq
	}
	return nil
}

func PackageDeserialize(data []byte) (*Package, error) {
	var pkg Package
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}
	return &pkg, nil
}

var reqId atomic.Uint32

func NewPackage(path string, body []byte, sid string, id int) *Package {
	if id == -1 {
		reqId.Add(1)
		if id = int(reqId.Load()); id == math.MaxUint32 {
			reqId.Store(0)
		}
	}
	return &Package{Id: uint32(id), Path: path, Sid: sid, Body: body}
}

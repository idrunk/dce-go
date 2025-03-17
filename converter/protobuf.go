package converter

import (
	"go.drunkce.com/dce/router"
	"go.drunkce.com/dce/util"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func PbRequester[Rp router.RoutableProtocol, ReqDto proto.Message, Req any](ctx *router.Context[Rp]) *router.Requester[Rp, ReqDto, Req] {
	return &router.Requester[Rp, ReqDto, Req]{Context: ctx, Deserializer: ProtobufDeserializer[ReqDto](0)}
}

func PbRawRequester[Rp router.RoutableProtocol, Req proto.Message](ctx *router.Context[Rp]) *router.Requester[Rp, Req, Req] {
	return PbRequester[Rp, Req, Req](ctx)
}

func PbStatusRequester[Rp router.RoutableProtocol](ctx *router.Context[Rp]) *router.Requester[Rp, *Status, router.Status] {
	return PbRequester[Rp, *Status, router.Status](ctx)
}


func PbResponser[Rp router.RoutableProtocol, Resp any, RespDto proto.Message](ctx *router.Context[Rp]) *router.Responser[Rp, Resp, RespDto] {
	return &router.Responser[Rp, Resp, RespDto]{Context: ctx, Serializer: ProtobufSerializer[RespDto](0)}
}

func PbRawResponser[Rp router.RoutableProtocol, Resp proto.Message](ctx *router.Context[Rp]) *router.Responser[Rp, Resp, Resp] {
	return PbResponser[Rp, Resp, Resp](ctx)
}

func PbStatusResponser[Rp router.RoutableProtocol](ctx *router.Context[Rp]) *router.Responser[Rp, router.Status, *Status] {
	return PbResponser[Rp, router.Status, *Status](ctx)
}


type ProtobufDeserializer[T proto.Message] uint8

func (p ProtobufDeserializer[T]) Deserialize(seq []byte) (T, error) {
	t := util.NewStruct[T]()
	err := proto.Unmarshal(seq, t)
	return t, err
}

type ProtobufSerializer[T proto.Message] uint8

func (p ProtobufSerializer[T]) Serialize(t T) ([]byte, error) {
	return proto.Marshal(t)
}



func PbJsonRequester[Rp router.RoutableProtocol, ReqDto proto.Message, Req any](ctx *router.Context[Rp]) *router.Requester[Rp, ReqDto, Req] {
	return &router.Requester[Rp, ReqDto, Req]{Context: ctx, Deserializer: ProtobufJsonDeserializer[ReqDto](0)}
}

func PbJsonRawRequester[Rp router.RoutableProtocol, Req proto.Message](ctx *router.Context[Rp]) *router.Requester[Rp, Req, Req] {
	return PbJsonRequester[Rp, Req, Req](ctx)
}

func PbJsonStatusRequester[Rp router.RoutableProtocol](ctx *router.Context[Rp]) *router.Requester[Rp, *Status, *router.Status] {
	return PbJsonRequester[Rp, *Status, *router.Status](ctx)
}


func PbJsonResponser[Rp router.RoutableProtocol, Resp any, RespDto proto.Message](ctx *router.Context[Rp]) *router.Responser[Rp, Resp, RespDto] {
	return &router.Responser[Rp, Resp, RespDto]{Context: ctx, Serializer: ProtobufJsonSerializer[RespDto](0)}
}

func PbJsonRawResponser[Rp router.RoutableProtocol, Resp proto.Message](ctx *router.Context[Rp]) *router.Responser[Rp, Resp, Resp] {
	return PbJsonResponser[Rp, Resp, Resp](ctx)
}

func PbJsonStatusResponser[Rp router.RoutableProtocol](ctx *router.Context[Rp]) *router.Responser[Rp, *router.Status, *Status] {
	return PbJsonResponser[Rp, *router.Status, *Status](ctx)
}

type ProtobufJsonDeserializer[T proto.Message] uint8

func (p ProtobufJsonDeserializer[T]) Deserialize(seq []byte) (T, error) {
	t := util.NewStruct[T]()
	err := protojson.Unmarshal(seq, t)
	return t, err
}

type ProtobufJsonSerializer[T proto.Message] uint8

func (p ProtobufJsonSerializer[T]) Serialize(t T) ([]byte, error) {
	return protojson.Marshal(t)
}


func (ps *Status) Into() (*router.Status, error) {
	s := router.Status{Data: ps.Data}
	if ps.Status != nil {
		s.Status = *ps.Status
	}
	if ps.Code != nil {
		s.Code = int(*ps.Code)
	}
	if ps.Msg != nil {
		s.Msg = *ps.Msg
	}
	return &s, nil
}

func (ps *Status) From(s *router.Status) (*Status, error) {
	p := &Status{}
	p.Status = &s.Status
	p.Code = util.Ref(int64(s.Code))
	p.Msg = &s.Msg
	p.Data = s.Data
	return p, nil
}

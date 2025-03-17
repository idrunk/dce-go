package converter

import (
	"encoding/json"

	"go.drunkce.com/dce/router"
)

func JsonRequester[Rp router.RoutableProtocol, ReqDto, Req any](ctx *router.Context[Rp]) *router.Requester[Rp, ReqDto, Req] {
	return &router.Requester[Rp, ReqDto, Req]{Context: ctx, Deserializer: JsonDeserializer[ReqDto](0)}
}

func JsonRawRequester[Rp router.RoutableProtocol, Req any](ctx *router.Context[Rp]) *router.Requester[Rp, Req, Req] {
	return JsonRequester[Rp, Req, Req](ctx)
}

func JsonStatusRequester[Rp router.RoutableProtocol](ctx *router.Context[Rp]) *router.Requester[Rp, *router.Status, *router.Status] {
	return JsonRawRequester[Rp, *router.Status](ctx)
}

func JsonMapRequester[Rp router.RoutableProtocol](ctx *router.Context[Rp]) *router.Requester[Rp, map[string]any, map[string]any] {
	return JsonRawRequester[Rp, map[string]any](ctx)
}


func JsonResponser[Rp router.RoutableProtocol, Resp, RespDto any](ctx *router.Context[Rp]) *router.Responser[Rp, Resp, RespDto] {
	return &router.Responser[Rp, Resp, RespDto]{Context: ctx, Serializer: JsonSerializer[RespDto](0)}
}

func JsonRawResponser[Rp router.RoutableProtocol, Resp any](ctx *router.Context[Rp]) *router.Responser[Rp, Resp, Resp] {
	return JsonResponser[Rp, Resp, Resp](ctx)
}

func JsonStatusResponser[Rp router.RoutableProtocol](ctx *router.Context[Rp]) *router.Responser[Rp, *router.Status, *router.Status] {
	return JsonRawResponser[Rp, *router.Status](ctx)
}

func JsonMapResponser[Rp router.RoutableProtocol](ctx *router.Context[Rp]) *router.Responser[Rp, map[string]any, map[string]any] {
	return JsonRawResponser[Rp, map[string]any](ctx)
}


type JsonDeserializer[T any] uint8

func (s JsonDeserializer[T]) Deserialize(seq []byte) (T, error) {
	var obj T
	err := json.Unmarshal(seq, &obj)
	return obj, err
}

type JsonSerializer[T any] uint8

func (s JsonSerializer[T]) Serialize(obj T) ([]byte, error) {
	return json.Marshal(obj)
}

package converter

import (
	"encoding/json"
	"github.com/idrunk/dce-go/router"
	"github.com/idrunk/dce-go/util"
)

type JsonRequestProcessor[Rp router.RoutableProtocol, ReqDto, Req, Resp, RespDto any] struct {
	*router.Context[Rp]
}

func (j *JsonRequestProcessor[Rp, ReqDto, Req, Resp, RespDto]) Serialize(dto RespDto) ([]byte, error) {
	return json.Marshal(dto)
}

func (j *JsonRequestProcessor[Rp, ReqDto, Req, Resp, RespDto]) Deserialize(seq []byte) (ReqDto, error) {
	var obj ReqDto
	err := json.Unmarshal(seq, &obj)
	return obj, err
}

func (j *JsonRequestProcessor[Rp, ReqDto, Req, Resp, RespDto]) Parse() (Req, bool) {
	bytes, err := j.Rp.Body()
	if err == nil {
		dto, err2 := j.Deserialize(bytes)
		if err2 == nil {
			obj, err3 := router.DtoInto[ReqDto, Req](dto)
			if err3 == nil {
				return obj, true
			}
			err = err3
		} else {
			err = err2
		}
	}
	j.Rp.SetError(err)
	var obj Req
	return obj, false
}

func (j *JsonRequestProcessor[Rp, ReqDto, Req, Resp, RespDto]) Response(obj Resp) bool {
	if dto, err := router.DtoFrom[Resp, RespDto](obj); err != nil {
		j.Rp.SetError(err)
	} else if seq, err := j.Serialize(dto); err != nil {
		j.Rp.SetError(err)
	} else if _, err := j.Rp.Write(seq); err != nil {
		j.Rp.SetError(err)
	}
	j.Rp.SetCtxData(router.HttpContentTypeKey, "application/json; charset=utf-8")
	return true
}

func (j *JsonRequestProcessor[Rp, ReqDto, Req, Resp, RespDto]) Error(err error) bool {
	j.Rp.SetError(err)
	code, msg := util.ResponseUnits(err)
	return j.Status(false, msg, code, nil)
}

func (j *JsonRequestProcessor[Rp, ReqDto, Req, Resp, RespDto]) Success(data any) bool {
	return j.Status(true, "", 0, data)
}

func (j *JsonRequestProcessor[Rp, ReqDto, Req, Resp, RespDto]) Fail(msg string, code int) bool {
	return j.Status(false, msg, code, nil)
}

func (j *JsonRequestProcessor[Rp, ReqDto, Req, Resp, RespDto]) Status(status bool, msg string, code int, data any) bool {
	if seq, err := json.Marshal(router.Status{Status: status, Msg: msg, Code: code, Data: data}); err != nil {
		j.Rp.SetError(err)
	} else if _, err := j.Rp.Write(seq); err != nil {
		j.Rp.SetError(err)
	}
	j.Rp.SetCtxData(router.HttpContentTypeKey, "application/json; charset=utf-8")
	return true
}

func JsonConverterNoConvert[Rp router.RoutableProtocol](c *router.Context[Rp]) JsonRequestProcessor[Rp, router.DoNotConvert, router.DoNotConvert, router.DoNotConvert, router.DoNotConvert] {
	return JsonRequestProcessor[Rp, router.DoNotConvert, router.DoNotConvert, router.DoNotConvert, router.DoNotConvert]{c}
}

func JsonConverterNoParseSame[Rp router.RoutableProtocol, Resp any](c *router.Context[Rp]) JsonRequestProcessor[Rp, router.DoNotConvert, router.DoNotConvert, Resp, Resp] {
	return JsonRequestProcessor[Rp, router.DoNotConvert, router.DoNotConvert, Resp, Resp]{c}
}

func JsonConverterNoParse[Rp router.RoutableProtocol, Resp, RespDto any](c *router.Context[Rp]) JsonRequestProcessor[Rp, router.DoNotConvert, router.DoNotConvert, Resp, RespDto] {
	return JsonRequestProcessor[Rp, router.DoNotConvert, router.DoNotConvert, Resp, RespDto]{c}
}

func JsonConverterSame[Rp router.RoutableProtocol, Req, Resp any](c *router.Context[Rp]) JsonRequestProcessor[Rp, Req, Req, Resp, Resp] {
	return JsonRequestProcessor[Rp, Req, Req, Resp, Resp]{c}
}

func JsonMapConverter[Rp router.RoutableProtocol](c *router.Context[Rp]) JsonRequestProcessor[Rp, map[string]any, map[string]any, map[string]any, map[string]any] {
	return JsonRequestProcessor[Rp, map[string]any, map[string]any, map[string]any, map[string]any]{c}
}

func JsonMapConverterNoParse[Rp router.RoutableProtocol](c *router.Context[Rp]) JsonRequestProcessor[Rp, router.DoNotConvert, router.DoNotConvert, map[string]any, map[string]any] {
	return JsonRequestProcessor[Rp, router.DoNotConvert, router.DoNotConvert, map[string]any, map[string]any]{c}
}

func JsonConverter[Rp router.RoutableProtocol, ReqDto, Req, Resp, RespDto any](c *router.Context[Rp]) JsonRequestProcessor[Rp, ReqDto, Req, Resp, RespDto] {
	return JsonRequestProcessor[Rp, ReqDto, Req, Resp, RespDto]{c}
}

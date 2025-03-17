package router

import (
	"reflect"

	"go.drunkce.com/dce/util"
)

type Requester[Rp RoutableProtocol, ReqDto, Req any] struct {
	Context *Context[Rp]
	Deserializer Deserializer[ReqDto]
}

func (r *Requester[Rp, ReqDto, Req]) Deserialize(seq []byte) (Req, error) {
	dto, err := r.Deserializer.Deserialize(seq)
	if err != nil {
		return util.NewStruct[Req](), err
	}
	return DtoInto[ReqDto, Req](dto)
}

func (r *Requester[Rp, ReqDto, Req]) Parse() (Req, bool) {
	if body, err := r.Context.Body(); err != nil {
		r.Context.Rp.SetError(err)
	} else if req, err := r.Deserialize(body); err != nil {
		r.Context.Rp.SetError(err)
	} else {
		return req, true
	}
	return util.NewStruct[Req](), false
}


type Responser[Rp RoutableProtocol, Resp, RespDto any] struct {
	Context *Context[Rp]
	Serializer Serializer[RespDto]
}

func (r *Responser[Rp, Resp, RespDto]) Serialize(obj Resp) ([]byte, error) {
	dto, err := DtoFrom[Resp, RespDto](obj)
	if err != nil {
		return nil, err
	}
	return r.Serializer.Serialize(dto)
}

func (r *Responser[Rp, Resp, RespDto]) Response(obj Resp) bool {
	if seq, err := r.Serialize(obj); err != nil {
		r.Context.Rp.SetError(err)
	} else if _, err := r.Context.Write(seq); err != nil {
		r.Context.Rp.SetError(err)
	}
	return true
}

func (r *Responser[Rp, Resp, RespDto]) Error(err error) bool {
	r.Context.Rp.SetError(err)
	code, msg := util.ResponseUnits(err)
	return r.Status(false, msg, code, nil)
}

func (r *Responser[Rp, Resp, RespDto]) Success(data *string) bool {
	return r.Status(true, "", 0, data)
}

func (r *Responser[Rp, Resp, RespDto]) Fail(msg string, code int) bool {
	return r.Status(false, msg, code, nil)
}

func (r *Responser[Rp, Resp, RespDto]) Status(status bool, msg string, code int, data *string) bool {
	if ! status && r.Context.Rp.Error() == nil {
		r.Context.Rp.SetError(util.Openly(code, "%s", msg))
	}
	if reflect.TypeFor[Resp]() == reflect.TypeFor[*Status]() {
		obj := &Status{Status: status, Msg: msg, Code: code, Data: data}
		return r.Response(any(obj).(Resp))
	} else if data != nil {
		r.Context.WriteString(*data)
	}
	return true
}

type Status struct {
	Status bool   	`json:"status,omitempty"`
	Code   int    	`json:"code,omitempty"`
	Msg    string 	`json:"msg,omitempty"`
	Data   *string  `json:"data,omitempty"`
}


type Serializer[T any] interface {
	Serialize(obj T) ([]byte, error)
}

type Deserializer[T any] interface {
	Deserialize(bytes []byte) (T, error)
}

type Into[T any] interface {
	Into() (T, error)
}

type From[S, T any] interface {
	From(src S) (T, error)
}

func DtoInto[Dto, Obj any](dto Dto) (Obj, error) {
	if obj, ok := any(dto).(Obj); ok {
		return obj, nil
	} else if dto, ok := any(dto).(Into[Obj]); ok {
		return dto.Into()
	}
	return util.NewStruct[Obj](), util.Closed0(`Type "%s" doesn't implement the "%s" interface`, reflect.TypeFor[Dto](), reflect.TypeFor[Into[Obj]]())
}

func DtoFrom[Obj, Dto any](obj Obj) (Dto, error) {
	if dto, ok := any(obj).(Dto); ok {
		return dto, nil
	}
	var emp Dto
	if dto, ok := any(emp).(From[Obj, Dto]); ok {
		return dto.From(obj)
	}
	return emp, util.Closed0(`Type "%s" doesn't implement the "%s" interface`, reflect.TypeFor[Dto](), reflect.TypeFor[From[Obj, Dto]]())
}

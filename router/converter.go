package router

import (
	"reflect"

	"github.com/idrunk/dce-go/util"
)

// RequestProcessor is a generic interface that defines the contract for processing requests.
// It is parameterized with two types: Obj and Dto.
//   - Obj represents the type of the object that will be processed.
//   - Dto represents the type of the Data Transfer Object (DTO) that will be used for serialization or deserialization.
// Implementations of this interface are expected to handle the processing logic for converting between Obj and Dto,
// and for managing the request lifecycle, including error handling and response generation.
type RequestProcessor[Obj, Dto any] interface {
	// Response processes the given response object of type Obj and returns a boolean indicating success or failure.
	// This method is typically used to handle the final output of a request processing pipeline.
	Response(resp Obj) bool

	// Error handles an error encountered during request processing and returns a boolean indicating whether the error was successfully handled.
	// This method is used to manage error states and ensure proper error reporting.
	Error(err error) bool

	// Success processes a successful response with the provided data and returns a boolean indicating success.
	// This method is used to handle successful outcomes and generate appropriate responses.
	Success(data any) bool

	// Fail processes a failure response with the provided error message and status code, and returns a boolean indicating failure.
	// This method is used to handle failed outcomes and generate appropriate error responses.
	Fail(msg string, code int) bool

	// Status processes a response with the provided status, message, status code, and data, and returns a boolean indicating success or failure.
	// This method is a more generalized version of Success and Fail, allowing for custom status handling.
	Status(status bool, msg string, code int, data any) bool
}

type Parser[Obj any] interface {
	Parse() (Obj, bool)
}

type Serializer[Dto any] interface {
	Serialize(dto Dto) ([]byte, error)
}

type Deserializer[Dto any] interface {
	Deserialize(bytes []byte) (Dto, error)
}

type Into[T any] interface {
	Into() (T, error)
}

type From[S, T any] interface {
	From(src S) (T, error)
}

func DtoInto[Dto, Obj any](dto Dto) (Obj, error) {
	if d, ok := any(dto).(Obj); ok {
		return d, nil
	} else if d2, ok2 := any(&dto).(Into[Obj]); ok2 {
		return d2.Into()
	}
	var obj Obj
	return obj, util.Closed0(`Type "%s" doesn't implement the "%s" interface`, reflect.TypeFor[Dto](), reflect.TypeFor[Into[Obj]]())
}

func DtoFrom[Obj, Dto any](obj Obj) (Dto, error) {
	dto := new(Dto)
	if d, ok := any(obj).(Dto); ok {
		return d, nil
	} else if d2, ok2 := any(dto).(From[Obj, Dto]); ok2 {
		return d2.From(obj)
	}
	return *dto, util.Closed0(`Type "%s" doesn't implement the "%s" interface`, reflect.TypeFor[Dto](), reflect.TypeFor[From[Obj, Dto]]())
}

type DoNotConvert uint8

type Status struct {
	Status bool   `json:"status,omitempty"`
	Code   int    `json:"code,omitempty"`
	Msg    string `json:"msg,omitempty"`
	Data   any    `json:"data,omitempty"`
}

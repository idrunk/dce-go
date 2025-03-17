package util

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"
)

const (
	ServiceUnavailable        = 503
	ServiceUnavailableMessage = "Service Unavailable"
)

const (
	ErrorSilent uint8 = iota
	ErrorOpenly
	ErrorClosed
)

type Error struct {
	Type    uint8
	Message string
	Code    int
}

func (e Error) Error() string {
	tyCode := ""
	switch e.Type {
	case ErrorOpenly:
		tyCode = "Openly"
	case ErrorClosed:
		tyCode = "Closed"
	}
	if e.Code != 0 {
		tyCode += " " + strconv.Itoa(e.Code)
	}
	return fmt.Sprintf("[%s %s] %s", time.Now().Format("2006-01-02 15:04:05"), tyCode, e.Message)
}

func (e Error) IsOpenly() bool {
	return e.Type == ErrorOpenly
}

func ResponseUnits(err error) (int, string) {
	var e Error
	if errors.As(err, &e) && e.Type == ErrorOpenly {
		return e.Code, e.Message
	}
	return ServiceUnavailable, ServiceUnavailableMessage
}

func newError(tp uint8, code int, msg string) Error {
	return Error{Type: tp, Message: msg, Code: code}
}

func Openly(code int, format string, args ...any) Error {
	return newError(ErrorOpenly, code, fmt.Sprintf(format, args...))
}

func Closed(code int, format string, args ...any) Error {
	return newError(ErrorClosed, code, fmt.Sprintf(format, args...))
}

func Openly0(format string, args ...any) Error {
	return Openly(0, format, args...)
}

func Closed0(format string, args ...any) Error {
	return Closed(0, format, args...)
}

func Silent(format string, args ...any) Error {
	return newError(ErrorSilent, 0, fmt.Sprintf(format, args...))
}

func Iif[T any](test bool, trueVal T, falseVal T) T {
	if test {
		return trueVal
	}
	return falseVal
}

func NewStruct[T any]() T {
	var t T
	ty := reflect.TypeOf(t)
	if ty.Kind() != reflect.Ptr {
		return t
	}
	v := reflect.New(ty.Elem())
	return v.Interface().(T)
}

func Ref[T any](v T) *T {
	return &v
}

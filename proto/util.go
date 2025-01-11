package proto

import (
	"github.com/idrunk/dce-go/router"
	"github.com/idrunk/dce-go/util"
	"log/slog"
	"sync"
)

func NewConnectorMappingManager[Rp router.RoutableProtocol, C any](routerId string) ConnectorMappingManager[Rp, C] {
	return ConnectorMappingManager[Rp, C]{router.ProtoRouter[Rp](routerId), util.NewStruct[sync.Map](), util.NewStruct[sync.Map]()}
}

type ConnectorMappingManager[Rp router.RoutableProtocol, C any] struct {
	*router.Router[Rp]
	connMapping sync.Map
	uidMapping  sync.Map
}

func (w *ConnectorMappingManager[Rp, C]) SetMapping(addr string, conn C) {
	w.connMapping.Store(addr, conn)
}

func (w *ConnectorMappingManager[Rp, C]) Unmapping(addr string) {
	w.connMapping.Delete(addr)
	w.UidUnmapping(addr)
}

func (w *ConnectorMappingManager[Rp, C]) Except(addr string, err error) bool {
	w.Unmapping(addr)
	slog.Debug("Client disconnected with: %s", err.Error())
	return false
}

func (w *ConnectorMappingManager[Rp, C]) Warn(err error) bool {
	slog.Warn(err.Error())
	return true
}

func (w *ConnectorMappingManager[Rp, C]) ConnMapping() map[string]C {
	cm := make(map[string]C)
	w.connMapping.Range(func(key, value interface{}) bool {
		cm[key.(string)] = value.(C)
		return true
	})
	return cm
}

func (w *ConnectorMappingManager[Rp, C]) ListBy(filter func(s string) bool) []util.Tuple2[string, C] {
	return util.MapSeq2From[string, C, util.Tuple2[string, C]](w.ConnMapping()).Filter2(func(s string, _ C) bool {
		return filter(s)
	}).Map2(func(addr string, conn C) util.Tuple2[string, C] {
		return util.NewTuple2(addr, conn)
	}).Collect()
}

func (w *ConnectorMappingManager[Rp, C]) ConnBy(addr string) (C, bool) {
	if conn, ok := w.connMapping.Load(addr); ok {
		return conn.(C), true
	}
	return util.NewStruct[C](), false
}

func (w *ConnectorMappingManager[Rp, C]) UidSetMapping(addr string, uid uint64) {
	w.uidMapping.Store(addr, uid)
}

func (w *ConnectorMappingManager[Rp, C]) UidUnmapping(addr string) {
	w.uidMapping.Delete(addr)
}

func (w *ConnectorMappingManager[Rp, C]) UidMapping() map[string]uint64 {
	um := make(map[string]uint64)
	w.uidMapping.Range(func(key, value interface{}) bool {
		um[key.(string)] = value.(uint64)
		return true
	})
	return um
}

package firevault

import (
	"reflect"
	"sync"
)

// holds parsed struct data
type structCache struct {
	sync.Map // map[reflect.Type]*structData
}

// contains name and each field's scope
type structData struct {
	name   string
	fields []*fieldScope
}

// get cache
func (sc *structCache) get(structType reflect.Type) (*structData, bool) {
	c, ok := sc.Load(structType)
	if !ok {
		return nil, false
	}

	return c.(*structData), true
}

// set cache
func (sc *structCache) set(structType reflect.Type, structData *structData) {
	sc.Store(structType, structData)
}

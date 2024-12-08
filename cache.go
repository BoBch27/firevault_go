package firevault

import (
	"reflect"
	"sync"
)

type structCache struct {
	sync.Map // map[reflect.Type]*structMetadata
}

type structMetadata struct {
	name   string
	fields []*fieldScope
}

// initialize with an empty map
func newStructCache() *structCache {
	return &structCache{}
}

func (sc *structCache) get(structType reflect.Type) (*structMetadata, bool) {
	c, ok := sc.Load(structType)
	if !ok {
		return nil, false
	}

	return c.(*structMetadata), true
}

func (sc *structCache) set(structType reflect.Type, structData *structMetadata) {
	sc.Store(structType, structData)
}

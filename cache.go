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
	fields []*fieldMetadata
}

type fieldMetadata struct {
	fieldScope
	idx    int
	ignore bool
	nested bool
	rules  []string
}

func (sc *structCache) Get(structType reflect.Type) (*structMetadata, bool) {
	val, found := sc.Load(structType)
	if found {
		return val.(*structMetadata), true
	}

	return nil, false
}

func (sc *structCache) Set(structType reflect.Type, structData *structMetadata) {
	sc.Store(structType, structData)
}

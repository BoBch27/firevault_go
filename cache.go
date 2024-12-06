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
	idx       int
	omitEmpty string
	nested    bool
	rules     []*tagMetadata
}

type tagMetadata struct {
	rule    string
	valFn   ValidationFn
	transFn TransformationFn
	param   string
	// keys                 *cTag // only populated when using tag's 'keys' and 'endkeys' for map key validation
	// next                 *cTag
	// typeof               tagType
	// hasParam bool // true if parameter used eg. eq= where the equal sign has been set
	// isBlockEnd           bool // indicates the current tag represents the last validation in the block
	isTransform bool
	runOnNil    bool
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

package firevault

import (
	"reflect"
	"sync"
	"sync/atomic"
)

type structCache struct {
	lock sync.Mutex
	m    atomic.Value // map[reflect.Type]*structMetadata
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
	rules  []*tagMetadata
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
	sc := &structCache{}
	sc.m.Store(make(map[reflect.Type]*structMetadata))
	return sc
}

func (sc *structCache) get(structType reflect.Type) (*structMetadata, bool) {
	m := sc.m.Load().(map[reflect.Type]*structMetadata)
	val, found := m[structType]
	return val, found
}

func (sc *structCache) set(structType reflect.Type, structData *structMetadata) {
	sc.lock.Lock()
	defer sc.lock.Unlock()

	// Get current map
	m := sc.m.Load().(map[reflect.Type]*structMetadata)

	// Create new map with increased capacity
	nm := make(map[reflect.Type]*structMetadata, len(m)+1)

	// Copy existing entries
	for k, v := range m {
		nm[k] = v
	}

	// Add new entry
	nm[structType] = structData

	// Atomically store new map
	sc.m.Store(nm)
}

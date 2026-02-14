package textarea

// Simple memoization cache for line wrapping with LRU eviction
type lineContent interface {
	Hash() string
}

type MemoCache[T lineContent, V any] struct {
	items    map[string]V
	maxItems int
}

func NewMemoCache[T lineContent, V any](maxItems int) *MemoCache[T, V] {
	return &MemoCache[T, V]{
		items:    make(map[string]V),
		maxItems: maxItems,
	}
}

func (m *MemoCache[T, V]) Get(key T) (V, bool) {
	val, ok := m.items[key.Hash()]
	if !ok {
		var zero V
		return zero, false
	}
	return val, true
}

func (m *MemoCache[T, V]) Set(key T, value V) {
	// If cache is full, clear it (simple eviction strategy)
	if len(m.items) >= m.maxItems {
		m.items = make(map[string]V)
	}
	m.items[key.Hash()] = value
}

func (m *MemoCache[T, V]) Capacity() int {
	return len(m.items)
}

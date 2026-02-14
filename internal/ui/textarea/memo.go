package textarea

// Simple memoization cache for line wrapping
type lineContent interface {
	Hash() string
}

type MemoCache[T lineContent, V any] struct {
	items map[string]V
}

func NewMemoCache[T lineContent, V any](maxItems int) *MemoCache[T, V] {
	return &MemoCache[T, V]{
		items: make(map[string]V),
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
	m.items[key.Hash()] = value
}

func (m *MemoCache[T, V]) Capacity() int {
	return len(m.items)
}

package topos

import "github.com/freehandle/breeze/crypto"

type MemoryBlock struct {
	epoch   uint64
	actions []int // end of n-th instruction
	data    []byte
}

func (b *MemoryBlock) Hash() crypto.Hash {
	return crypto.Hasher(b.data)
}

func (b *MemoryBlock) Len() int {
	return len(b.actions)
}

func (b *MemoryBlock) Get(n int) []byte {
	if n >= len(b.actions) || n < 0 {
		return nil
	}
	starts := 0
	if n > 0 {
		starts = b.actions[n-1]
	}
	ends := b.actions[n]
	return b.data[starts+1 : ends]
}

func (b *MemoryBlock) Append(data []byte) {
	b.data = append(b.data, data...)
	b.actions = append(b.actions, len(b.data))
}

func (m *MemoryBlock) Clone() *MemoryBlock {
	data := make([]byte, len(m.data))
	copy(data, m.data)
	actions := make([]int, len(m.actions))
	copy(actions, m.actions)
	return &MemoryBlock{
		epoch:   m.epoch,
		actions: actions,
		data:    data,
	}
}

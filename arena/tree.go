package arena

import (
	"sync/atomic"
	"unsafe"
)

type memTree struct {
	root unsafe.Pointer
}

type memNode struct {
	Addr      uintptr
	Size      uintptr
	OwnerMask uint64
	Left      unsafe.Pointer
	Right     unsafe.Pointer
}

func (t *memTree) insert(addr, size uintptr, ownerMask uint64) *memNode {
	if ownerMask == 0 {
		ownerMask = 1
	}
	newNode := &memNode{Addr: addr, Size: size, OwnerMask: ownerMask}
	for {
		root := (*memNode)(atomic.LoadPointer(&t.root))
		if root == nil {
			if atomic.CompareAndSwapPointer(&t.root, nil, unsafe.Pointer(newNode)) {
				return newNode
			}
			continue
		}
		if existing := insertNode(root, newNode); existing != nil {
			return existing
		}
	}
}

func insertNode(root, newNode *memNode) *memNode {
	cur := root
	for {
		if newNode.Addr == cur.Addr {
			atomic.OrUint64(&cur.OwnerMask, newNode.OwnerMask)
			return cur
		}
		link := &cur.Left
		if newNode.Addr > cur.Addr {
			link = &cur.Right
		}
		next := (*memNode)(atomic.LoadPointer(link))
		if next == nil {
			if atomic.CompareAndSwapPointer(link, nil, unsafe.Pointer(newNode)) {
				return newNode
			}
			continue
		}
		cur = next
	}
}

func (t *memTree) find(addr uintptr) *memNode {
	cur := (*memNode)(atomic.LoadPointer(&t.root))
	for cur != nil {
		switch {
		case addr == cur.Addr:
			return cur
		case addr < cur.Addr:
			cur = (*memNode)(atomic.LoadPointer(&cur.Left))
		default:
			cur = (*memNode)(atomic.LoadPointer(&cur.Right))
		}
	}
	return nil
}

func (t *memTree) freeVarying(addr uintptr, layerMask uint64) (remaining uint64, found bool) {
	node := t.find(addr)
	if node == nil {
		return 0, false
	}
	for {
		current := atomic.LoadUint64(&node.OwnerMask)
		next := current &^ layerMask
		if atomic.CompareAndSwapUint64(&node.OwnerMask, current, next) {
			return next, true
		}
	}
}

package balloc

import (
	"sync/atomic"
	"unsafe"
)

type BallocNode struct {
	Addr      uintptr
	Size      uintptr
	OwnerMask uint64
	Left      unsafe.Pointer
	Right     unsafe.Pointer
}

type freeSlot struct {
	addr uintptr
	size uintptr
	next unsafe.Pointer
}

func insertNode(root *unsafe.Pointer, node *BallocNode) *BallocNode {
	for {
		current := (*BallocNode)(atomic.LoadPointer(root))
		if current == nil {
			if atomic.CompareAndSwapPointer(root, nil, unsafe.Pointer(node)) {
				return node
			}
			continue
		}
		return insertBelow(current, node)
	}
}

func insertBelow(root, node *BallocNode) *BallocNode {
	current := root
	for {
		if node.Addr == current.Addr {
			atomic.StoreUintptr(&current.Size, node.Size)
			atomic.OrUint64(&current.OwnerMask, node.OwnerMask)
			return current
		}
		link := &current.Left
		if node.Addr > current.Addr {
			link = &current.Right
		}
		next := (*BallocNode)(atomic.LoadPointer(link))
		if next == nil {
			if atomic.CompareAndSwapPointer(link, nil, unsafe.Pointer(node)) {
				return node
			}
			continue
		}
		current = next
	}
}

func findNode(root unsafe.Pointer, addr uintptr) *BallocNode {
	current := (*BallocNode)(atomic.LoadPointer(&root))
	for current != nil {
		switch {
		case addr == current.Addr:
			return current
		case addr < current.Addr:
			current = (*BallocNode)(atomic.LoadPointer(&current.Left))
		default:
			current = (*BallocNode)(atomic.LoadPointer(&current.Right))
		}
	}
	return nil
}

func clearOwner(node *BallocNode, layerMask uint64) uint64 {
	for {
		current := atomic.LoadUint64(&node.OwnerMask)
		next := current &^ layerMask
		if atomic.CompareAndSwapUint64(&node.OwnerMask, current, next) {
			return next
		}
	}
}

func pushFree(root *unsafe.Pointer, slot *freeSlot) {
	for {
		head := atomic.LoadPointer(root)
		slot.next = head
		if atomic.CompareAndSwapPointer(root, head, unsafe.Pointer(slot)) {
			return
		}
	}
}

func popFree(root *unsafe.Pointer, size uintptr) (uintptr, uintptr, bool) {
	for {
		head := (*freeSlot)(atomic.LoadPointer(root))
		if head == nil {
			return 0, 0, false
		}
		next := atomic.LoadPointer(&head.next)
		if !atomic.CompareAndSwapPointer(root, unsafe.Pointer(head), next) {
			continue
		}
		if head.size >= size {
			return head.addr, head.size, true
		}
		pushFree(root, head)
		return 0, 0, false
	}
}

package main

import "container/list"

type entry struct{ key, val int }

type LRUCache struct {
	cap   int
	order *list.List
	items map[int]*list.Element
}

func Constructor(capacity int) LRUCache {
	return LRUCache{cap: capacity, order: list.New(), items: map[int]*list.Element{}}
}

func (this *LRUCache) Get(key int) int {
	if e, ok := this.items[key]; ok {
		this.order.MoveToFront(e)
		return e.Value.(*entry).val
	}
	return -1
}

func (this *LRUCache) Put(key int, value int) {
	if e, ok := this.items[key]; ok {
		e.Value.(*entry).val = value
		this.order.MoveToFront(e)
		return
	}
	this.items[key] = this.order.PushFront(&entry{key, value})
	if this.order.Len() > this.cap {
		last := this.order.Back()
		this.order.Remove(last)
		delete(this.items, last.Value.(*entry).key)
	}
}

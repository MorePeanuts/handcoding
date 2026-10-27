package main

type NestedIterator struct {
	vals []int
}

func Constructor(nestedList []*NestedInteger) *NestedIterator {
	// Flatten everything up front.
	it := &NestedIterator{}
	var walk func(list []*NestedInteger)
	walk = func(list []*NestedInteger) {
		for _, n := range list {
			if n.IsInteger() {
				it.vals = append(it.vals, n.GetInteger())
			} else {
				walk(n.GetList())
			}
		}
	}
	walk(nestedList)
	return it
}

func (this *NestedIterator) Next() int {
	v := this.vals[0]
	this.vals = this.vals[1:]
	return v
}

func (this *NestedIterator) HasNext() bool {
	return len(this.vals) > 0
}

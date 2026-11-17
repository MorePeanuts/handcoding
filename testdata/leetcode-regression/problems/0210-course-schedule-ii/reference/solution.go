package main

// findOrder is a depth-first search that adds each course once all courses
// depending on it are added, then reverses: another valid order than the
// Python reference's.
func findOrder(numCourses int, prerequisites [][]int) []int {
	after := make([][]int, numCourses)
	for _, p := range prerequisites {
		after[p[1]] = append(after[p[1]], p[0])
	}
	const (
		unseen = iota
		active
		finished
	)
	state := make([]int, numCourses)
	order := make([]int, 0, numCourses)
	type frame struct{ course, next int }
	// Iterative, since a chain of courses may be 2000 long.
	for start := 0; start < numCourses; start++ {
		if state[start] != unseen {
			continue
		}
		stack := []frame{{start, 0}}
		state[start] = active
		for len(stack) > 0 {
			top := &stack[len(stack)-1]
			if top.next == len(after[top.course]) {
				state[top.course] = finished
				order = append(order, top.course)
				stack = stack[:len(stack)-1]
				continue
			}
			c := after[top.course][top.next]
			top.next++
			switch state[c] {
			case active:
				return []int{}
			case unseen:
				state[c] = active
				stack = append(stack, frame{c, 0})
			}
		}
	}
	for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
		order[i], order[j] = order[j], order[i]
	}
	return order
}

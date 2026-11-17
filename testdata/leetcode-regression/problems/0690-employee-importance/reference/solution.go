package main

func getImportance(employees []*Employee, id int) int {
	byID := map[int]*Employee{}
	for _, e := range employees {
		byID[e.Id] = e
	}
	var total func(id int) int
	total = func(id int) int {
		e := byID[id]
		sum := e.Importance
		for _, s := range e.Subordinates {
			sum += total(s)
		}
		return sum
	}
	return total(id)
}

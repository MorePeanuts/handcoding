package main

func numIslands(grid [][]byte) int {
	var sink func(i, j int)
	sink = func(i, j int) {
		if i < 0 || j < 0 || i >= len(grid) || j >= len(grid[0]) || grid[i][j] != '1' {
			return
		}
		grid[i][j] = '0'
		sink(i+1, j)
		sink(i-1, j)
		sink(i, j+1)
		sink(i, j-1)
	}
	count := 0
	for i := range grid {
		for j := range grid[i] {
			if grid[i][j] == '1' {
				count++
				sink(i, j)
			}
		}
	}
	return count
}

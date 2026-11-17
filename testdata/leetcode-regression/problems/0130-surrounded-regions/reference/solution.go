package main

func solve(board [][]byte) {
	m, n := len(board), len(board[0])
	var keep func(i, j int)
	keep = func(i, j int) {
		if i < 0 || i >= m || j < 0 || j >= n || board[i][j] != 'O' {
			return
		}
		board[i][j] = '#'
		keep(i+1, j)
		keep(i-1, j)
		keep(i, j+1)
		keep(i, j-1)
	}
	for i := 0; i < m; i++ {
		keep(i, 0)
		keep(i, n-1)
	}
	for j := 0; j < n; j++ {
		keep(0, j)
		keep(m-1, j)
	}
	for _, row := range board {
		for j, c := range row {
			if c == '#' {
				row[j] = 'O'
			} else {
				row[j] = 'X'
			}
		}
	}
}

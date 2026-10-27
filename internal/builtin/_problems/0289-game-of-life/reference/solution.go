package main

func gameOfLife(board [][]int) {
	// In place: bit 0 is the current state, bit 1 the next one.
	m, n := len(board), len(board[0])
	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			live := 0
			for x := max(0, i-1); x <= min(m-1, i+1); x++ {
				for y := max(0, j-1); y <= min(n-1, j+1); y++ {
					if x != i || y != j {
						live += board[x][y] & 1
					}
				}
			}
			if live == 3 || live == 2 && board[i][j]&1 == 1 {
				board[i][j] |= 2
			}
		}
	}
	for _, row := range board {
		for j := range row {
			row[j] >>= 1
		}
	}
}

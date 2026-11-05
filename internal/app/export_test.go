package app

import "time"

// SetTimeLimit sets the factor and floor of the time limit, which tests
// lower so that sleeping solutions show it quickly.
func (a *App) SetTimeLimit(factor int, floor time.Duration) {
	a.timeFactor, a.timeFloor = factor, floor
}

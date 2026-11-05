package app

import "time"

// SetTimeLimit sets the factor of the time limit and the floor of the
// reference's time, which tests lower so that sleeping solutions show it
// quickly.
func (a *App) SetTimeLimit(factor int, floor time.Duration) {
	a.timeFactor, a.baselineFloor = factor, floor
}

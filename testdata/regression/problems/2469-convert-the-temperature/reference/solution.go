package main

func convertTemperature(celsius float64) []float64 {
	// Computed in another order than the Python reference, so the two differ
	// in the last bits and the cases rely on the tolerance.
	kelvin := (celsius*100 + 27315) / 100
	fahrenheit := (celsius*9 + 160) / 5
	return []float64{kelvin, fahrenheit}
}

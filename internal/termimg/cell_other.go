//go:build !unix

package termimg

import "io"

func cellRatio(io.Writer) float64 { return defaultCellRatio }

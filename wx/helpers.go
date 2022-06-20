package wx

import (
	"math"
)

func Dewpoint(rh, t float64) (td float64) {
	rr := (17.625 * t) / (243.04 + t)
	lrh := math.Log(rh / 100)
	return 243.04 * (lrh + rr) / (17.625 - lrh - rr)
}

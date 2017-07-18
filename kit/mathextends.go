package kit

import (
	"math"
)

//Округление до целого числа
func Round(val float64, roundOn float64, exactness int) (newVal float64) {
	var round float64
	pow := math.Pow(10, float64(exactness))
	digit := pow * val
	_, div := math.Modf(digit)
	if div >= roundOn {
		round = math.Ceil(digit)
	} else {
		round = math.Floor(digit)
	}
	newVal = round / pow
	return
}

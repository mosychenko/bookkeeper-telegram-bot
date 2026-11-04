package budget

import (
	"math"
	"strconv"
	"strings"
)

// FormatRUB renders a whole-ruble amount with a regular space as the
// thousands separator: 46234 -> "46 234 ₽".
func FormatRUB(n int64) string {
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	b.Grow(len(sign) + len(s) + len(s)/3 + 4)
	b.WriteString(sign)
	for i := 0; i < len(s); i++ {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteByte(s[i])
	}
	b.WriteString(" ₽")
	return b.String()
}

// RoundDiv rounds n/d to the nearest integer, half away from zero.
func RoundDiv(n, d int64) int64 {
	if d == 0 {
		return 0
	}
	return int64(math.Round(float64(n) / float64(d)))
}

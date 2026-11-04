package budget

import (
	"fmt"
	"time"
)

func MonthNominative(m time.Month) string {
	return [...]string{
		"", "Январь", "Февраль", "Март", "Апрель", "Май", "Июнь",
		"Июль", "Август", "Сентябрь", "Октябрь", "Ноябрь", "Декабрь",
	}[m]
}

func MonthGenitive(m time.Month) string {
	return [...]string{
		"", "января", "февраля", "марта", "апреля", "мая", "июня",
		"июля", "августа", "сентября", "октября", "ноября", "декабря",
	}[m]
}

func MonthPrepositional(m time.Month) string {
	return [...]string{
		"", "январе", "феврале", "марте", "апреле", "мае", "июне",
		"июле", "августе", "сентябре", "октябре", "ноябре", "декабре",
	}[m]
}

// MonthAdjective is the capitalized form used in "Ноябрьский период".
func MonthAdjective(m time.Month) string {
	return [...]string{
		"", "Январский", "Февральский", "Мартовский", "Апрельский", "Майский", "Июньский",
		"Июльский", "Августовский", "Сентябрьский", "Октябрьский", "Ноябрьский", "Декабрьский",
	}[m]
}

func FormatDayMonth(t time.Time) string {
	return fmt.Sprintf("%d.%d", t.Day(), int(t.Month()))
}

func FormatDayMonthPadded(t time.Time) string {
	return fmt.Sprintf("%02d.%02d", t.Day(), int(t.Month()))
}

func daysWord(n int) string {
	a := n % 100
	if a < 0 {
		a = -a
	}
	b := a % 10
	if a > 10 && a < 20 {
		return "дней"
	}
	if b == 1 {
		return "день"
	}
	if b >= 2 && b <= 4 {
		return "дня"
	}
	return "дней"
}

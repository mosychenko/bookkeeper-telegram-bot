package budget

import (
	"fmt"
	"time"
)

// Kind is which payday funds the period.
type Kind string

const (
	KindSalary  Kind = "salary"  // 5th through the day before the advance
	KindAdvance Kind = "advance" // advance day through the day before the next salary
)

// Period is one half of the month: salary (5th-19th) or advance (20th-4th).
type Period struct {
	Kind       Kind
	Start      time.Time // inclusive
	End        time.Time // inclusive
	NextPayday time.Time
	Days       int
	// Year and Month are the funding payday's month. That is the monthly
	// sheet tab: November for the 5 Nov salary, October for the 20 Oct advance.
	Year  int
	Month time.Month
}

// PeriodContaining returns the period that contains t.
// The calendar day is taken in t's location, so pass Moscow time in production.
func PeriodContaining(t time.Time, salaryDay, advanceDay int) (Period, error) {
	if salaryDay < 1 || advanceDay < 1 || salaryDay > 28 || advanceDay > 28 || salaryDay >= advanceDay {
		return Period{}, fmt.Errorf("некорректные дни выплат: зарплата %d, аванс %d", salaryDay, advanceDay)
	}
	y, m, d := t.Date()
	switch {
	case d >= advanceDay:
		start := date(y, m, advanceDay)
		end := date(y, m, salaryDay).AddDate(0, 1, -1)
		next := date(y, m, salaryDay).AddDate(0, 1, 0)
		return newPeriod(KindAdvance, start, end, next, y, m), nil
	case d < salaryDay:
		prev := date(y, m, 1).AddDate(0, -1, 0)
		start := date(prev.Year(), prev.Month(), advanceDay)
		end := date(y, m, salaryDay-1)
		next := date(y, m, salaryDay)
		return newPeriod(KindAdvance, start, end, next, prev.Year(), prev.Month()), nil
	default:
		start := date(y, m, salaryDay)
		end := date(y, m, advanceDay-1)
		next := date(y, m, advanceDay)
		return newPeriod(KindSalary, start, end, next, y, m), nil
	}
}

// PeriodOf builds the salary or advance period whose funding payday falls in
// the given month. Used by the fixture, which is pinned to a prototype month.
func PeriodOf(year int, month time.Month, kind Kind, salaryDay, advanceDay int) (Period, error) {
	var probe time.Time
	switch kind {
	case KindSalary:
		probe = date(year, month, salaryDay)
	case KindAdvance:
		probe = date(year, month, advanceDay)
	default:
		return Period{}, fmt.Errorf("неизвестный период %q", kind)
	}
	p, err := PeriodContaining(probe, salaryDay, advanceDay)
	if err != nil {
		return Period{}, err
	}
	if p.Kind != kind || p.Month != month || p.Year != year {
		return Period{}, fmt.Errorf("период %s %s %d не собрался", kind, month, year)
	}
	return p, nil
}

// Previous is the same kind of period one month earlier.
func (p Period) Previous(salaryDay, advanceDay int) (Period, error) {
	return PeriodContaining(p.Start.AddDate(0, -1, 0), salaryDay, advanceDay)
}

// PreviousFundingMonth is the calendar month before the funding month.
func (p Period) PreviousFundingMonth() (int, time.Month) {
	t := date(p.Year, p.Month, 1).AddDate(0, -1, 0)
	return t.Year(), t.Month()
}

func (p Period) Label() string {
	return FormatDayMonth(p.Start) + " → " + FormatDayMonth(p.End)
}

func (p Period) IncomeWord() string {
	if p.Kind == KindAdvance {
		return "аванс"
	}
	return "зарплата"
}

func newPeriod(kind Kind, start, end, next time.Time, year int, month time.Month) Period {
	return Period{
		Kind:       kind,
		Start:      start,
		End:        end,
		NextPayday: next,
		Days:       inclusiveDays(start, end),
		Year:       year,
		Month:      month,
	}
}

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func DateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func inclusiveDays(a, b time.Time) int {
	d := int(b.Sub(a).Hours()/24) + 1
	if d < 1 {
		return 0
	}
	return d
}

// InRange reports whether the calendar day of t is inside [start, end].
func InRange(t, start, end time.Time) bool {
	d := DateOnly(t)
	return !d.Before(DateOnly(start)) && !d.After(DateOnly(end))
}

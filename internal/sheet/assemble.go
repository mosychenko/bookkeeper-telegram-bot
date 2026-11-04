package sheet

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mosychenko/bookkeeper-telegram-bot/internal/budget"
)

// Assemble turns raw grids into the calculator input. It only reads; the
// caller is responsible for fetching the ranges described by Mapping.
// `now` picks the period (pass Moscow time). The "data as of" date is the
// latest operation date, so a stale sheet is visible on the screen.
func Assemble(now time.Time, bf BudgetFile, mp Mapping, opsGrid, curGrid, prevGrid [][]string) (budget.Input, error) {
	mp = mp.normalized()
	if err := bf.Validate(); err != nil {
		return budget.Input{}, err
	}
	if err := mp.Validate(); err != nil {
		return budget.Input{}, err
	}
	if err := checkHeaders(opsGrid, mp.Operations.HeaderRow, mp.Operations.ExpectHeaders); err != nil {
		return budget.Input{}, fmt.Errorf("лист %q: %w", mp.Operations.Sheet, err)
	}
	if err := checkHeaders(curGrid, mp.Monthly.HeaderRow, mp.Monthly.ExpectHeaders); err != nil {
		return budget.Input{}, fmt.Errorf("месячный лист: %w", err)
	}
	if err := checkHeaders(prevGrid, mp.Monthly.HeaderRow, mp.Monthly.ExpectHeaders); err != nil {
		return budget.Input{}, fmt.Errorf("лист прошлого месяца: %w", err)
	}

	ops, warns := parseOperations(opsGrid, mp)
	cur := parseMonthly(curGrid, mp)
	prev := parseMonthly(prevGrid, mp)

	period, err := budget.PeriodContaining(now, bf.Payday.SalaryDay, bf.Payday.AdvanceDay)
	if err != nil {
		return budget.Input{}, err
	}
	prevPeriod, err := period.Previous(bf.Payday.SalaryDay, bf.Payday.AdvanceDay)
	if err != nil {
		return budget.Input{}, err
	}

	asOf := budget.DateOnly(now)
	var sawDate bool
	for _, op := range ops {
		if !op.DateOK {
			continue
		}
		if !sawDate || op.Date.After(asOf) {
			asOf = op.Date
			sawDate = true
		}
	}
	if !sawDate {
		warns = append(warns, "В операциях нет дат, дата данных — сегодня.")
	} else if asOf.After(budget.DateOnly(now)) {
		asOf = budget.DateOnly(now)
	}

	planCol := "plan_salary"
	incomeName := mp.Monthly.IncomeRows.Salary
	incomeDefault := bf.Income.Salary
	if period.Kind == budget.KindAdvance {
		planCol = "plan_advance"
		incomeName = mp.Monthly.IncomeRows.Advance
		incomeDefault = bf.Income.Advance
	}

	planned, plannedKnown := incomeFrom(cur, incomeName, planCol)
	if !plannedKnown {
		planned = incomeDefault
		warns = append(warns, fmt.Sprintf("Приход «%s» в листе не найден, взято %s из настроек. Сверьте имя строки.", incomeName, budget.FormatRUB(planned)))
	}
	lastIncome, lastKnown := incomeFrom(prev, incomeName, planCol)

	var mandatory []budget.Payment
	for _, item := range bf.ForPeriod(period.Kind) {
		amount, fromSheet := amountFromRow(cur[norm(item.Category)], planCol)
		if !fromSheet {
			if item.Optional && item.AmountDefault == 0 {
				continue
			}
			amount = item.AmountDefault
			warns = append(warns, fmt.Sprintf("«%s»: в листе нет суммы, взято %s из настроек.", item.Name, budget.FormatRUB(amount)))
		}
		if item.Optional && amount == 0 {
			continue
		}
		p := budget.Payment{
			Name:   item.Name,
			Amount: amount,
			Due:    item.DueLabel(period.Month),
		}
		if note, ok := paidNote(ops, bf, item.Category, period); ok {
			p.Paid = true
			p.PaidNote = note
		}
		mandatory = append(mandatory, p)
	}

	var flexible []budget.Flex
	for _, item := range bf.Flexible {
		row := cur[norm(item.Category)]
		prevRow := prev[norm(item.Category)]
		var plan *int64
		switch planCol {
		case "plan_advance":
			plan = row.PlanAdvance
		default:
			plan = row.PlanSalary
		}
		fact := factOf(mp.FactSource, row, ops, item.Category, period.Start, period.End, mp.Types)
		var last int64
		if mp.LastMonthFact == "operations" {
			py, pm := period.PreviousFundingMonth()
			start, end := monthSpan(py, pm)
			last = sumCategory(ops, item.Category, start, end, mp.Types)
		} else if prevRow.Fact != nil {
			last = *prevRow.Fact
		}
		if plan == nil && fact == 0 && last == 0 {
			continue
		}
		flexible = append(flexible, budget.Flex{
			Name:      item.Category,
			Plan:      plan,
			Fact:      fact,
			LastMonth: last,
		})
	}

	pace, paceKnown, paceWarn := paceFrom(ops, bf, prev, prevPeriod, mp)
	if paceWarn != "" {
		warns = append(warns, paceWarn)
	}

	hasSpend := sumAll(ops, period.Start, period.End, mp.Types) > 0
	usePrev, _, _ := budget.TopMeta(now, period, hasSpend)
	var top []budget.Spent
	if usePrev {
		py, pm := period.PreviousFundingMonth()
		start, end := monthSpan(py, pm)
		top = rank(ops, start, end, mp.Types)
		if len(top) == 0 {
			top = rankMonthly(prev)
		}
	} else {
		top = rank(ops, period.Start, period.End, mp.Types)
	}

	return budget.Input{
		Now:             now,
		AsOf:            asOf,
		Period:          period,
		Planned:         planned,
		LastIncome:      lastIncome,
		LastIncomeKnown: lastKnown,
		LastFlexPerDay:  pace,
		PaceKnown:       paceKnown,
		Mandatory:       mandatory,
		Flexible:        flexible,
		Top:             top,
		HasPeriodSpend:  hasSpend,
		Warnings:        warns,
	}, nil
}

func incomeFrom(rows map[string]CatRow, name, planCol string) (int64, bool) {
	if strings.TrimSpace(name) == "" {
		return 0, false
	}
	row, ok := rows[norm(name)]
	if !ok {
		return 0, false
	}
	return amountFromRow(row, planCol)
}

func amountFromRow(row CatRow, planCol string) (int64, bool) {
	var p *int64
	if planCol == "plan_advance" {
		p = row.PlanAdvance
	} else {
		p = row.PlanSalary
	}
	if p == nil {
		return 0, false
	}
	return *p, true
}

func factOf(source string, row CatRow, ops []Operation, category string, start, end time.Time, types TypeNames) int64 {
	if source == "operations" {
		return sumCategory(ops, category, start, end, types)
	}
	if row.Fact != nil {
		return *row.Fact
	}
	return 0
}

func paidNote(ops []Operation, bf BudgetFile, category string, period budget.Period) (string, bool) {
	// Look at the funding month, not only the period: a bill dated the 1st
	// can still belong to the salary period (subscriptions in the prototype).
	start, end := monthSpan(period.Year, period.Month)
	for _, op := range ops {
		if !same(op.BudgetCategory, category) || !bf.paid(op.Status) {
			continue
		}
		if !op.DateOK || !budget.InRange(op.Date, start, end) {
			continue
		}
		amount := op.Amount
		if op.FactOK {
			amount = op.Fact
		}
		amount = abs64(amount)
		desc := op.Description
		if desc == "" {
			desc = op.Comment
		}
		if desc == "" {
			return budget.FormatRUB(amount), true
		}
		return budget.FormatRUB(amount) + " " + desc, true
	}
	return "", false
}

func sumCategory(ops []Operation, category string, start, end time.Time, types TypeNames) int64 {
	var n int64
	for _, op := range ops {
		if !same(op.BudgetCategory, category) {
			continue
		}
		if op.DateOK && !budget.InRange(op.Date, start, end) {
			continue
		}
		if !op.DateOK {
			continue
		}
		if v, ok := op.spend(types); ok {
			n += v
		}
	}
	return n
}

func sumAll(ops []Operation, start, end time.Time, types TypeNames) int64 {
	var n int64
	for _, op := range ops {
		if !op.DateOK || !budget.InRange(op.Date, start, end) {
			continue
		}
		if v, ok := op.spend(types); ok {
			n += v
		}
	}
	return n
}

func sumFlexible(ops []Operation, bf BudgetFile, start, end time.Time, types TypeNames) int64 {
	var n int64
	for _, item := range bf.Flexible {
		n += sumCategory(ops, item.Category, start, end, types)
	}
	return n
}

func paceFrom(ops []Operation, bf BudgetFile, prev map[string]CatRow, prevPeriod budget.Period, mp Mapping) (int64, bool, string) {
	spent := sumFlexible(ops, bf, prevPeriod.Start, prevPeriod.End, mp.Types)
	if spent > 0 && prevPeriod.Days > 0 {
		return budget.RoundDiv(spent, int64(prevPeriod.Days)), true, ""
	}
	var monthly int64
	var any bool
	for _, item := range bf.Flexible {
		row := prev[norm(item.Category)]
		if row.Fact != nil {
			monthly += *row.Fact
			any = true
		}
	}
	if any && prevPeriod.Days > 0 && monthly > 0 {
		return budget.RoundDiv(monthly, int64(prevPeriod.Days)), true,
			"Темп прошлого периода посчитан по факту месяца, а не по дням того же периода. Сверьте с операциями."
	}
	return 0, false, ""
}

func rank(ops []Operation, start, end time.Time, types TypeNames) []budget.Spent {
	type acc struct {
		name string
		sum  int64
	}
	by := map[string]*acc{}
	for _, op := range ops {
		if !op.DateOK || !budget.InRange(op.Date, start, end) {
			continue
		}
		v, ok := op.spend(types)
		if !ok || v == 0 || strings.TrimSpace(op.BudgetCategory) == "" {
			continue
		}
		key := norm(op.BudgetCategory)
		if by[key] == nil {
			by[key] = &acc{name: strings.TrimSpace(op.BudgetCategory)}
		}
		by[key].sum += v
	}
	list := make([]budget.Spent, 0, len(by))
	for _, a := range by {
		list = append(list, budget.Spent{Name: a.name, Amount: a.sum})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Amount != list[j].Amount {
			return list[i].Amount > list[j].Amount
		}
		return list[i].Name < list[j].Name
	})
	if len(list) > 5 {
		list = list[:5]
	}
	return list
}

func rankMonthly(rows map[string]CatRow) []budget.Spent {
	by := map[string]*struct {
		name string
		sum  int64
	}{}
	for _, row := range rows {
		if row.Fact == nil || *row.Fact <= 0 {
			continue
		}
		by[norm(row.Name)] = &struct {
			name string
			sum  int64
		}{name: row.Name, sum: *row.Fact}
	}
	list := make([]budget.Spent, 0, len(by))
	for _, a := range by {
		list = append(list, budget.Spent{Name: a.name, Amount: a.sum})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Amount != list[j].Amount {
			return list[i].Amount > list[j].Amount
		}
		return list[i].Name < list[j].Name
	})
	if len(list) > 5 {
		list = list[:5]
	}
	return list
}

func monthSpan(year int, month time.Month) (time.Time, time.Time) {
	start := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, -1)
	return start, end
}

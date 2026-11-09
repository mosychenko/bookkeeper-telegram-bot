package sheet

import (
	"fmt"
	"sort"
	"strconv"
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
	balance, balanceKnown, balanceAsOf, balanceDateOK, err := actualBalance(curGrid, mp)
	if err != nil {
		return budget.Input{}, err
	}
	if !balanceKnown {
		if mp.Monthly.Balance.Row == 0 {
			warns = append(warns, "Баланс в настройках листа не задан. Свободные считаются от планового дохода.")
		} else {
			cell := strings.ToUpper(strings.TrimSpace(mp.Monthly.Balance.Column)) + strconv.Itoa(mp.Monthly.Balance.Row)
			warns = append(warns, fmt.Sprintf("Баланс в ячейке %s пустой или не число. Свободные считаются от планового дохода.", cell))
		}
	} else if balanceDateOK && budget.DateOnly(balanceAsOf).Before(budget.DateOnly(now)) {
		warns = append(warns, fmt.Sprintf("Баланс в листе от %s, сегодня %s. Свободные считаются от этой цифры.", budget.FormatDayMonth(balanceAsOf), budget.FormatDayMonth(now)))
	}
	planned, plannedKnown, err := plannedIncome(curGrid, mp, period.Kind)
	if err != nil {
		return budget.Input{}, err
	}
	if mp.Monthly.IncomePlan.Row == 0 {
		planned, plannedKnown = incomeFrom(cur, incomeName, planCol)
	}
	if !plannedKnown {
		planned = incomeDefault
		label := incomeName
		if strings.TrimSpace(label) == "" {
			label = "периода"
		}
		warns = append(warns, fmt.Sprintf("Приход «%s» в листе не найден, взято %s из настроек.", label, budget.FormatRUB(planned)))
	}
	lastIncome, lastKnown, _, err := salaryInPeriod(ops, prevGrid, bf, mp, prevPeriod)
	if err != nil {
		return budget.Input{}, err
	}
	if !actualConfigured(mp, period.Kind) && !hasSalaryDetail(ops, bf, mp.Types.Income) {
		lastIncome, lastKnown = incomeFrom(prev, incomeName, planCol)
	}
	curSalary, curKnown, curRent, err := salaryInPeriod(ops, curGrid, bf, mp, period)
	if err != nil {
		return budget.Input{}, err
	}

	var mandatory []budget.Payment
	for _, item := range bf.Mandatory {
		if !item.allows(period.Kind) {
			continue
		}
		if item.ID == "savings" {
			amount := savingsAmount(curSalary, curKnown, planned, curRent, bf.SavingsRate)
			if amount == 0 {
				continue
			}
			p := budget.Payment{
				Name:   item.Name,
				Amount: amount,
				Due:    item.DueLabel(period.Month, period.Kind),
			}
			// C3 already has the whole piggy bank taken out: 10% of salary
			// and rent. Subtracting this line again would count it twice.
			// The fallback to planned income still counts the full line.
			if balanceKnown {
				p.CoveredByBalance = amount
			}
			mandatory = append(mandatory, p)
			continue
		}
		amount, fromSheet := amountFromRow(cur[norm(item.Category)], planCol)
		// An empty cell means this category is not in this period.
		// The other period has its own column. A blank fill is used only
		// when the config explicitly sets one (see advance_if_blank).
		if !fromSheet || amount == 0 {
			if fill := item.blankFill(period.Kind); fill > 0 {
				amount = fill
			} else {
				continue
			}
		}
		p := budget.Payment{
			Name:   item.Name,
			Amount: amount,
			Due:    item.DueLabel(period.Month, period.Kind),
		}
		if note, ok := paidNote(ops, bf, item.opsCategory(), period); ok {
			p.Paid = true
			p.PaidNote = note
		}
		mandatory = append(mandatory, p)
	}

	var flexible []budget.Flex
	for _, item := range bf.flexItems() {
		row := cur[norm(item.Category)]
		prevRow := prev[norm(item.Category)]
		var plan *int64
		switch planCol {
		case "plan_advance":
			plan = row.PlanAdvance
		default:
			plan = row.PlanSalary
		}
		fact := factOf(mp.FactSource, row, ops, item.opsCategory(), period.Start, period.End, mp.Types)
		var last int64
		if mp.LastMonthFact == "operations" {
			py, pm := period.PreviousFundingMonth()
			start, end := monthSpan(py, pm)
			last = sumCategory(ops, item.opsCategory(), start, end, mp.Types)
		} else if prevRow.Fact != nil {
			last = *prevRow.Fact
		}
		// Rare categories stay in the pace and cover math, but the screen
		// hides them until this period has its own plan or fact.
		if item.Rare {
			if plan == nil && fact == 0 {
				continue
			}
		} else if plan == nil && fact == 0 && last == 0 {
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
		Balance:         balance,
		BalanceKnown:    balanceKnown,
		BalanceAsOf:     balanceAsOf,
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

func actualBalance(grid [][]string, mp Mapping) (amount int64, ok bool, synced time.Time, dateOK bool, err error) {
	b := mp.Monthly.Balance
	if b.Row == 0 {
		return 0, false, time.Time{}, false, nil
	}
	if err := expectLabel(grid, b.Row, b.LabelColumn, b.Label); err != nil {
		return 0, false, time.Time{}, false, fmt.Errorf("баланс по счетам: %w", err)
	}
	n, moneyOK := parseMoney(gridCell(grid, b.Row, b.Column))
	if !moneyOK {
		return 0, false, time.Time{}, false, nil
	}
	synced, dateOK = parseDate(gridCell(grid, b.Row, b.SyncDate))
	return n, true, synced, dateOK, nil
}

func plannedIncome(grid [][]string, mp Mapping, kind budget.Kind) (int64, bool, error) {
	ip := mp.Monthly.IncomePlan
	if ip.Row == 0 {
		return 0, false, nil
	}
	if err := expectLabel(grid, ip.Row, ip.LabelColumn, ip.Label); err != nil {
		return 0, false, fmt.Errorf("плановый доход: %w", err)
	}
	col := ip.SalaryColumn
	if kind == budget.KindAdvance {
		col = ip.AdvanceColumn
	}
	n, ok := parseMoney(gridCell(grid, ip.Row, col))
	return n, ok, nil
}

func actualConfigured(mp Mapping, kind budget.Kind) bool {
	cell := mp.Monthly.IncomeActual.Salary
	if kind == budget.KindAdvance {
		cell = mp.Monthly.IncomeActual.Advance
	}
	return cell.Row > 0 && strings.TrimSpace(cell.Column) != ""
}

func actualIncome(grid [][]string, mp Mapping, kind budget.Kind) (int64, bool, error) {
	if !actualConfigured(mp, kind) {
		return 0, false, nil
	}
	cell := mp.Monthly.IncomeActual.Salary
	if kind == budget.KindAdvance {
		cell = mp.Monthly.IncomeActual.Advance
	}
	if err := expectLabel(grid, cell.Row, cell.LabelColumn, cell.Label); err != nil {
		return 0, false, fmt.Errorf("фактический доход: %w", err)
	}
	n, ok := parseMoney(gridCell(grid, cell.Row, cell.Column))
	return n, ok, nil
}

func expectLabel(grid [][]string, row int, col, want string) error {
	want = normalizeWS(want)
	if want == "" {
		return nil
	}
	if strings.TrimSpace(col) == "" {
		col = "A"
	}
	got := normalizeWS(gridCell(grid, row, col))
	if got != want {
		return fmt.Errorf("ячейка %s%d: в таблице %q, ожидается %q", strings.ToUpper(strings.TrimSpace(col)), row, got, want)
	}
	return nil
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
	for _, item := range bf.flexItems() {
		n += sumCategory(ops, item.opsCategory(), start, end, types)
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
	for _, item := range bf.flexItems() {
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

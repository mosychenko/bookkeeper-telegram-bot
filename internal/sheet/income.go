package sheet

import (
	"math"
	"time"

	"github.com/mosychenko/bookkeeper-telegram-bot/internal/budget"
)

// salaryInPeriod is the salary-category inflow of one period.
// Operations win when the sheet has any salary or excluded income row,
// because only dates can split «Доход Женя» between the 5th and the 20th.
// Otherwise the monthly J block is used: J3 (plus J5) for the salary
// period and J4 for the advance period. «Прочие поступления» and loans
// are never added.
func salaryInPeriod(ops []Operation, grid [][]string, bf BudgetFile, mp Mapping, period budget.Period) (amount int64, known bool, rent int64, err error) {
	if err := ignoreNonSalary(grid, mp); err != nil {
		return 0, false, 0, err
	}
	if hasSalaryDetail(ops, bf, mp.Types.Income) {
		amount = sumIncome(ops, bf.SalaryIncomeCategories, period.Start, period.End, mp.Types.Income, false)
		rent = sumIncome(ops, bf.ExcludedIncomeCategories, period.Start, period.End, mp.Types.Income, true)
		return amount, true, rent, nil
	}
	amount, known, err = salaryFromBlock(grid, mp, period.Kind)
	return amount, known, 0, err
}

// savingsAmount is 10% of the period's salary once it has arrived, otherwise
// 10% of that period's plan (C3 or D3). Rent is added in full on top.
func savingsAmount(actual int64, known bool, planned, rent int64, rate float64) int64 {
	base := planned
	if known && actual > 0 {
		base = actual
	}
	part := int64(math.Round(rate * float64(base)))
	if part < 0 {
		part = 0
	}
	if rent < 0 {
		rent = 0
	}
	return part + rent
}

func hasSalaryDetail(ops []Operation, bf BudgetFile, incomeType string) bool {
	for _, op := range ops {
		if !same(op.Type, incomeType) || !op.DateOK {
			continue
		}
		if bf.listed(bf.SalaryIncomeCategories, op.BudgetCategory) || bf.listed(bf.ExcludedIncomeCategories, op.BudgetCategory) {
			return true
		}
	}
	return false
}

func sumIncome(ops []Operation, categories []string, start, end time.Time, incomeType string, absolute bool) int64 {
	var sum int64
	for _, op := range ops {
		if !same(op.Type, incomeType) || !op.DateOK || !budget.InRange(op.Date, start, end) {
			continue
		}
		if !listedName(categories, op.BudgetCategory) {
			continue
		}
		amount, ok := incomeAmount(op)
		if !ok {
			continue
		}
		if absolute {
			if amount < 0 {
				amount = -amount
			}
		}
		sum += amount
	}
	return sum
}

func listedName(categories []string, name string) bool {
	n := norm(name)
	if n == "" {
		return false
	}
	for _, c := range categories {
		if norm(c) == n {
			return true
		}
	}
	return false
}

func incomeAmount(op Operation) (int64, bool) {
	if op.AmountOK {
		return op.Amount, true
	}
	if op.FactOK {
		return op.Fact, true
	}
	return 0, false
}

func salaryFromBlock(grid [][]string, mp Mapping, kind budget.Kind) (int64, bool, error) {
	primary, ok, err := actualIncome(grid, mp, kind)
	if err != nil {
		return 0, false, err
	}
	total := int64(0)
	known := false
	if ok {
		total += primary
		known = true
	}
	if kind == budget.KindSalary && mp.Monthly.IncomeActual.Wife.Row > 0 {
		extra, extraOK, err := readLabeled(grid, mp.Monthly.IncomeActual.Wife)
		if err != nil {
			return 0, false, err
		}
		if extraOK {
			total += extra
			known = true
		}
	}
	return total, known, nil
}

// ignoreNonSalary checks the labels of inflows that must not become free
// money. The amounts themselves are not returned.
func ignoreNonSalary(grid [][]string, mp Mapping) error {
	if _, _, err := readLabeled(grid, mp.Monthly.IncomeActual.Other); err != nil {
		return err
	}
	if _, _, err := readLabeled(grid, mp.Monthly.IncomeActual.Loan); err != nil {
		return err
	}
	return nil
}

func readLabeled(grid [][]string, cell IncomeCell) (int64, bool, error) {
	if cell.Row == 0 || cell.Column == "" {
		return 0, false, nil
	}
	if err := expectLabel(grid, cell.Row, cell.LabelColumn, cell.Label); err != nil {
		return 0, false, err
	}
	n, ok := parseMoney(gridCell(grid, cell.Row, cell.Column))
	return n, ok, nil
}

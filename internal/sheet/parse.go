package sheet

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Operation is one bank row. Money is whole rubles.
type Operation struct {
	Date           time.Time
	DateOK         bool
	Month          string
	Description    string
	Comment        string
	Amount         int64
	AmountOK       bool
	Type           string
	BudgetCategory string
	Fact           int64
	FactOK         bool
	Status         string
	Savings        string
}

// CatRow is one category on a monthly tab.
// Nil plan pointers mean the cell was empty (limit not set), not zero.
type CatRow struct {
	Name        string
	PlanSalary  *int64
	PlanAdvance *int64
	Fact        *int64
}

func parseOperations(grid [][]string, m Mapping) ([]Operation, []string) {
	var warns []string
	start := m.Operations.HeaderRow // 1-based header, data is the next row; index == header row
	if start < 0 {
		start = 0
	}
	var out []Operation
	badDates := 0
	for i := start; i < len(grid); i++ {
		row := grid[i]
		if rowEmpty(row) {
			continue
		}
		op := Operation{
			Month:          cellAt(row, m, "ops", "month"),
			Description:    cellAt(row, m, "ops", "description"),
			Comment:        cellAt(row, m, "ops", "comment"),
			Type:           cellAt(row, m, "ops", "type"),
			BudgetCategory: cellAt(row, m, "ops", "budget_category"),
			Status:         cellAt(row, m, "ops", "status"),
			Savings:        cellAt(row, m, "ops", "savings"),
		}
		if raw := cellAt(row, m, "ops", "date"); raw != "" {
			if t, ok := parseDate(raw); ok {
				op.Date, op.DateOK = t, true
			} else {
				badDates++
			}
		}
		if raw := cellAt(row, m, "ops", "amount"); raw != "" {
			if n, ok := parseMoney(raw); ok {
				op.Amount, op.AmountOK = n, true
			}
		}
		if raw := cellAt(row, m, "ops", "fact"); raw != "" {
			if n, ok := parseMoney(raw); ok {
				op.Fact, op.FactOK = n, true
			}
		}
		out = append(out, op)
	}
	if badDates > 0 {
		warns = append(warns, fmt.Sprintf("В операциях не разобралась дата у %d строк.", badDates))
	}
	return out, warns
}

func parseMonthly(grid [][]string, m Mapping) map[string]CatRow {
	out := map[string]CatRow{}
	start := m.Monthly.FirstDataRow - 1
	if start < 0 {
		start = 0
	}
	for i := start; i < len(grid); i++ {
		row := grid[i]
		name := cellAt(row, m, "month", "category")
		if name == "" {
			continue
		}
		cr := CatRow{Name: name}
		cr.PlanSalary = moneyPtr(cellAt(row, m, "month", "plan_salary"))
		cr.PlanAdvance = moneyPtr(cellAt(row, m, "month", "plan_advance"))
		cr.Fact = moneyPtr(cellAt(row, m, "month", "fact"))
		out[norm(name)] = cr
	}
	return out
}

func checkHeaders(grid [][]string, headerRow int, expect map[string]string) error {
	if len(expect) == 0 {
		return nil
	}
	if headerRow < 1 || len(grid) < headerRow {
		return fmt.Errorf("нет строки заголовков %d", headerRow)
	}
	row := grid[headerRow-1]
	for letter, want := range expect {
		idx, err := colIndex(letter)
		if err != nil {
			return err
		}
		got := ""
		if idx < len(row) {
			got = strings.TrimSpace(row[idx])
		}
		if got != strings.TrimSpace(want) {
			return fmt.Errorf("колонка %s: в таблице %q, в настройках ожидается %q", letter, got, strings.TrimSpace(want))
		}
	}
	return nil
}

func cellAt(row []string, m Mapping, group, key string) string {
	idx, err := m.col(group, key)
	if err != nil || idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func colIndex(letter string) (int, error) {
	letter = strings.ToUpper(strings.TrimSpace(letter))
	if letter == "" {
		return 0, fmt.Errorf("пустая буква колонки")
	}
	n := 0
	for _, c := range letter {
		if c < 'A' || c > 'Z' {
			return 0, fmt.Errorf("колонка %q", letter)
		}
		n = n*26 + int(c-'A'+1)
	}
	return n - 1, nil
}

func rowEmpty(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

func moneyPtr(raw string) *int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "—" || raw == "-" {
		return nil
	}
	n, ok := parseMoney(raw)
	if !ok {
		return nil
	}
	return &n
}

func parseMoney(raw string) (int64, bool) {
	s := strings.TrimSpace(raw)
	s = strings.ReplaceAll(s, "\u00a0", "")
	s = strings.ReplaceAll(s, "\u202f", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.TrimSuffix(s, "₽")
	s = strings.TrimSuffix(s, "р")
	s = strings.TrimSuffix(s, "р.")
	s = strings.ReplaceAll(s, ",", ".")
	if s == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return int64(math.Round(f)), true
}

func parseDate(raw string) (time.Time, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}, false
	}
	layouts := []string{
		"02.01.2006",
		"2.01.2006",
		"2.1.2006",
		"02.01.06",
		"2006-01-02",
		"02/01/2006",
		"2/1/2006",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return budgetDate(t), true
		}
	}
	// Google Sheets serial (days since 1899-12-30) when the API returns a number.
	if f, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64); err == nil && f > 20000 && f < 80000 {
		base := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
		return base.AddDate(0, 0, int(f)), true
	}
	return time.Time{}, false
}

func budgetDate(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func (o Operation) spend(types TypeNames) (int64, bool) {
	if same(o.Type, types.Excluded) || same(o.Type, types.Income) || same(o.Type, types.Loan) {
		return 0, false
	}
	spendish := same(o.Type, types.Expense) || same(o.Type, types.Cash) || (o.FactOK && o.Fact != 0)
	if !spendish {
		return 0, false
	}
	if o.FactOK {
		return abs64(o.Fact), true
	}
	if o.AmountOK {
		return abs64(o.Amount), true
	}
	return 0, false
}

func same(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

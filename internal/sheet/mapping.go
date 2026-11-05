package sheet

import "fmt"

// Mapping is the only place that knows which tab and column is which.
// It follows the live «Бюджет» workbook: two header rows on a monthly
// tab, plan-from-salary in C, plan-from-advance in D, fact in E.
type Mapping struct {
	Operations    OperationsMap `yaml:"operations"`
	Monthly       MonthlyMap    `yaml:"monthly"`
	Types         TypeNames     `yaml:"types"`
	FactSource    string        `yaml:"fact_source"`     // monthly | operations
	LastMonthFact string        `yaml:"last_month_fact"` // monthly | operations
}

type OperationsMap struct {
	Sheet         string            `yaml:"sheet"`
	HeaderRow     int               `yaml:"header_row"`
	Columns       map[string]string `yaml:"columns"`
	ExpectHeaders map[string]string `yaml:"expect_headers"`
	ReadColumns   string            `yaml:"read_columns"`
}

type MonthlyMap struct {
	HeaderRow     int               `yaml:"header_row"`
	FirstDataRow  int               `yaml:"first_data_row"`
	Columns       map[string]string `yaml:"columns"`
	ExpectHeaders map[string]string `yaml:"expect_headers"`
	ReadRange     string            `yaml:"read_range"`
	// IncomeRows is the legacy lookup by category name. The live sheet
	// keeps planned income on one row (income_plan) and actual income in
	// the I/J block (income_actual). Those win when their row is set.
	IncomeRows struct {
		Salary  string `yaml:"salary"`
		Advance string `yaml:"advance"`
	} `yaml:"income_rows"`
	IncomePlan   IncomePlan   `yaml:"income_plan"`
	IncomeActual IncomeActual `yaml:"income_actual"`
}

// IncomePlan is the «Доход» row: salary plan and advance plan are two
// columns of the same row, not two category rows.
type IncomePlan struct {
	Row           int    `yaml:"row"`
	Label         string `yaml:"label"`
	LabelColumn   string `yaml:"label_column"`
	SalaryColumn  string `yaml:"salary_column"`
	AdvanceColumn string `yaml:"advance_column"`
}

// IncomeActual is the «Доход факт (выписка)» block.
// Other and Loan are checked so a shifted column is noticed, and then ignored:
// «Прочие поступления» and «Заём у мамы» are not salary.
type IncomeActual struct {
	Salary  IncomeCell `yaml:"salary"`
	Advance IncomeCell `yaml:"advance"`
	Wife    IncomeCell `yaml:"wife"`
	Other   IncomeCell `yaml:"other"`
	Loan    IncomeCell `yaml:"loan"`
}

// IncomeCell is one labelled amount, for example J3 next to I3.
type IncomeCell struct {
	Row         int    `yaml:"row"`
	Column      string `yaml:"column"`
	Label       string `yaml:"label"`
	LabelColumn string `yaml:"label_column"`
}

type TypeNames struct {
	Expense  string `yaml:"expense"`
	Excluded string `yaml:"excluded"`
	Income   string `yaml:"income"`
	Loan     string `yaml:"loan"`
	Cash     string `yaml:"cash"`
}

func LoadMapping(path string) (Mapping, error) {
	var m Mapping
	if err := readYAML(path, &m); err != nil {
		return m, err
	}
	if m.FactSource == "" {
		m.FactSource = "monthly"
	}
	if m.LastMonthFact == "" {
		m.LastMonthFact = "monthly"
	}
	if err := m.Validate(); err != nil {
		return m, err
	}
	return m, nil
}

func (m Mapping) normalized() Mapping {
	if m.FactSource == "" {
		m.FactSource = "monthly"
	}
	if m.LastMonthFact == "" {
		m.LastMonthFact = "monthly"
	}
	return m
}

func (m Mapping) Validate() error {
	if m.Operations.Sheet == "" {
		return fmt.Errorf("operations.sheet пуст")
	}
	if m.Operations.HeaderRow < 1 {
		return fmt.Errorf("operations.header_row")
	}
	for _, key := range []string{"date", "amount", "type", "budget_category", "fact", "status", "description"} {
		if m.Operations.Columns[key] == "" {
			return fmt.Errorf("operations.columns.%s не задана", key)
		}
	}
	if m.Monthly.HeaderRow < 1 || m.Monthly.FirstDataRow <= m.Monthly.HeaderRow {
		return fmt.Errorf("monthly: строки заголовка и данных")
	}
	for _, key := range []string{"category", "plan_salary", "plan_advance", "fact"} {
		if m.Monthly.Columns[key] == "" {
			return fmt.Errorf("monthly.columns.%s не задана — сверьте config/sheets.yaml с книгой", key)
		}
	}
	if m.FactSource != "monthly" && m.FactSource != "operations" {
		return fmt.Errorf("fact_source: %s", m.FactSource)
	}
	if m.LastMonthFact != "monthly" && m.LastMonthFact != "operations" {
		return fmt.Errorf("last_month_fact: %s", m.LastMonthFact)
	}
	if m.Operations.ReadColumns == "" {
		return fmt.Errorf("operations.read_columns пуст")
	}
	if m.Monthly.ReadRange == "" {
		return fmt.Errorf("monthly.read_range пуст")
	}
	return nil
}

func (m Mapping) col(group, key string) (int, error) {
	var letter string
	switch group {
	case "ops":
		letter = m.Operations.Columns[key]
	case "month":
		letter = m.Monthly.Columns[key]
	default:
		return 0, fmt.Errorf("группа %s", group)
	}
	return colIndex(letter)
}

package sheet

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mosychenko/bookkeeper-telegram-bot/internal/budget"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// Values reads ranges. The production implementation calls only
// spreadsheets.values.get with the readonly scope.
type Values interface {
	Get(ctx context.Context, spreadsheetID, a1 string) ([][]string, error)
}

type googleValues struct {
	svc *sheets.Service
}

// NewGoogleValues builds a read-only Sheets client from service-account JSON.
func NewGoogleValues(ctx context.Context, credsJSON []byte) (Values, error) {
	creds, err := google.CredentialsFromJSON(ctx, credsJSON, sheets.SpreadsheetsReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("учётные данные Google: %w", err)
	}
	svc, err := sheets.NewService(ctx, option.WithCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("клиент Google Sheets: %w", err)
	}
	return &googleValues{svc: svc}, nil
}

func (g *googleValues) Get(ctx context.Context, spreadsheetID, a1 string) ([][]string, error) {
	resp, err := g.svc.Spreadsheets.Values.Get(spreadsheetID, a1).
		ValueRenderOption("FORMATTED_VALUE").
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("чтение %s: %w", a1, err)
	}
	out := make([][]string, len(resp.Values))
	for i, row := range resp.Values {
		out[i] = make([]string, len(row))
		for j, cell := range row {
			out[i][j] = fmt.Sprint(cell)
		}
	}
	return out, nil
}

// GoogleSource loads the period that contains now.
type GoogleSource struct {
	SpreadsheetID string
	Values        Values
	Budget        BudgetFile
	Map           Mapping
}

func (g *GoogleSource) Load(ctx context.Context, now time.Time) (budget.Input, error) {
	if g.Values == nil {
		return budget.Input{}, fmt.Errorf("клиент таблицы не настроен")
	}
	period, err := budget.PeriodContaining(now, g.Budget.Payday.SalaryDay, g.Budget.Payday.AdvanceDay)
	if err != nil {
		return budget.Input{}, err
	}
	ops, err := g.Values.Get(ctx, g.SpreadsheetID, a1(g.Map.Operations.Sheet, g.Map.Operations.ReadColumns))
	if err != nil {
		return budget.Input{}, err
	}
	curName := budget.MonthNominative(period.Month)
	_, prevMonth := period.PreviousFundingMonth()
	prevName := budget.MonthNominative(prevMonth)
	cur, err := g.Values.Get(ctx, g.SpreadsheetID, a1(curName, g.Map.Monthly.ReadRange))
	if err != nil {
		return budget.Input{}, fmt.Errorf("лист %s: %w", curName, err)
	}
	prev, err := g.Values.Get(ctx, g.SpreadsheetID, a1(prevName, g.Map.Monthly.ReadRange))
	if err != nil {
		return budget.Input{}, fmt.Errorf("лист %s: %w", prevName, err)
	}
	return Assemble(now, g.Budget, g.Map, ops, cur, prev)
}

func a1(sheet, cells string) string {
	return "'" + strings.ReplaceAll(sheet, "'", "''") + "'!" + cells
}

// FixtureSource serves the baked prototype and ignores the clock.
type FixtureSource struct {
	Input budget.Input
}

func (f FixtureSource) Load(context.Context, time.Time) (budget.Input, error) {
	return f.Input, nil
}

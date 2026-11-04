package sheet

import (
	"os"
	"strings"
	"testing"
)

func TestGoogleClientDoesNotWrite(t *testing.T) {
	b, err := os.ReadFile("google.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, bad := range []string{"Values.Update", "Values.Append", "Values.Clear", "BatchUpdate", "Spreadsheets.Create"} {
		if strings.Contains(src, bad) {
			t.Fatalf("google.go содержит %s — таблицу можно только читать", bad)
		}
	}
	if !strings.Contains(src, "SpreadsheetsReadonlyScope") {
		t.Fatal("нет scope только на чтение")
	}
	if !strings.Contains(src, "Values.Get") {
		t.Fatal("нет чтения диапазона")
	}
}

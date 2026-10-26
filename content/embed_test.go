package content

import (
	"testing"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/sheets"
)

// Встроенные шпаргалки проходят проверку схемы: битый файл ловится тестом, а не в рантайме.
func TestEmbeddedSheetsAreValid(t *testing.T) {
	all, err := sheets.LoadAll(Sheets)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) == 0 {
		t.Fatal("нет ни одной шпаргалки")
	}
}

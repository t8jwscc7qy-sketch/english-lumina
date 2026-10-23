package words

import "testing"

func TestFindMatch(t *testing.T) {
	run := [][]string{{"run"}, {"runs"}, {"running"}, {"ran"}}
	tests := []struct {
		name       string
		text       string
		forms      [][]string
		start, end int
		ok         bool
	}{
		{"форма слова в середине", "He is running fast.", run, 6, 13, true},
		{"регистр не важен", "RUNNING is fun", run, 0, 7, true},
		{"самое раннее совпадение", "Run, run, run", [][]string{{"run"}}, 0, 3, true},
		{"фраза", "Please turn to page ten.", [][]string{{"turn", "to"}}, 7, 14, true},
		{"фраза длиннее одного слова на той же позиции", "turn to me", [][]string{{"turn"}, {"turn", "to"}}, 0, 7, true},
		{"юникод: смещения в символах, а не в байтах", "I love café au lait", [][]string{{"café"}}, 7, 11, true},
		{"кириллица до слова не сбивает смещения", "Мы любим run каждый день", [][]string{{"run"}}, 9, 12, true},
		{"апостроф делит слово, как в полнотекстовом поиске", "I don't run", [][]string{{"don"}}, 2, 5, true},
		{"цифры", "Room 101 is big", [][]string{{"101"}}, 5, 8, true},
		{"часть слова не считается", "I like running", [][]string{{"run"}}, 0, 0, false},
		{"нет совпадения", "Nothing here", run, 0, 0, false},
		{"нет форм", "He runs", nil, 0, 0, false},
		{"пустой текст", "", run, 0, 0, false},
		{"фраза обрывается в конце текста", "I turn", [][]string{{"turn", "to"}}, 0, 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start, end, ok := FindMatch(tc.text, tc.forms)
			if ok != tc.ok || (ok && (start != tc.start || end != tc.end)) {
				t.Fatalf("FindMatch(%q) = (%d, %d, %v), ожидали (%d, %d, %v)", tc.text, start, end, ok, tc.start, tc.end, tc.ok)
			}
		})
	}
}

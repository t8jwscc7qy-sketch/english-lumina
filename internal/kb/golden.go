package kb

// GoldenCase — контрольное слово: допустимые основные русские переводы (нормализованные, `NormalizeRu`). Список
// написан автором и проверяется выборочно пользователем; сборка считает долю совпадений основного перевода
// (слова, которых нет в сборке, в долю не входят).
type GoldenCase struct {
	Lemma  string
	Pos    string
	Accept []string
}

// Golden — 40 контрольных слов, включая известные проблемные (dad, kid, thin, clothes, blow).
var Golden = []GoldenCase{
	{"dad", "noun", []string{"папа", "отец"}},
	{"kid", "noun", []string{"ребёнок", "ребенок", "малыш", "дитя"}},
	{"thin", "adjective", []string{"тонкий", "худой"}},
	{"clothes", "noun", []string{"одежда"}},
	{"blow", "verb", []string{"дуть", "дунуть"}},
	{"light", "noun", []string{"свет"}},
	{"table", "noun", []string{"стол"}},
	{"fire", "noun", []string{"огонь", "пожар"}},
	{"bank", "noun", []string{"банк"}},
	{"cat", "noun", []string{"кот", "кошка"}},
	{"dog", "noun", []string{"собака", "пёс"}},
	{"house", "noun", []string{"дом"}},
	{"water", "noun", []string{"вода"}},
	{"book", "noun", []string{"книга"}},
	{"apple", "noun", []string{"яблоко"}},
	{"big", "adjective", []string{"большой"}},
	{"small", "adjective", []string{"маленький"}},
	{"good", "adjective", []string{"хороший"}},
	{"bad", "adjective", []string{"плохой"}},
	{"happy", "adjective", []string{"счастливый", "весёлый", "радостный"}},
	{"run", "verb", []string{"бежать", "бегать"}},
	{"eat", "verb", []string{"есть", "кушать"}},
	{"drink", "verb", []string{"пить"}},
	{"go", "verb", []string{"идти", "ходить", "ехать", "пойти"}},
	{"see", "verb", []string{"видеть", "смотреть"}},
	{"know", "verb", []string{"знать"}},
	{"think", "verb", []string{"думать"}},
	{"love", "verb", []string{"любить"}},
	{"love", "noun", []string{"любовь"}},
	{"friend", "noun", []string{"друг", "подруга"}},
	{"mother", "noun", []string{"мать", "мама"}},
	{"father", "noun", []string{"отец", "папа"}},
	{"child", "noun", []string{"ребёнок", "ребенок", "дитя"}},
	{"school", "noun", []string{"школа"}},
	{"teacher", "noun", []string{"учитель", "учительница", "преподаватель"}},
	{"work", "verb", []string{"работать"}},
	{"city", "noun", []string{"город"}},
	{"money", "noun", []string{"деньги"}},
	{"time", "noun", []string{"время"}},
	{"morning", "noun", []string{"утро"}},
}

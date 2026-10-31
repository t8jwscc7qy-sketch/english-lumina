package library

import (
	"strings"
	"testing"
	"testing/fstest"
)

func okText(id string) string {
	return `{"id":"` + id + `","title":"Т","band":100,"targets":[{"lemma":"dog","pos":"noun"},{"lemma":"big","pos":"adjective"},{"lemma":"run","pos":"verb"}],"names":["Tom"],
"sentences":[{"en":"Tom has a big dog.","ru":"а"},{"en":"The big dog runs.","ru":"б"},{"en":"The dog runs to Tom and the big dog runs.","ru":"в"},{"en":"Tom likes the dog a lot, and Tom likes to run.","ru":"г"},{"en":"A man with a dog is here, and the dog is big.","ru":"д"}],"sources":["s"]}`
}

func TestParseAcceptsValidText(t *testing.T) {
	tx, err := Parse([]byte(okText("t100-001")))
	if err != nil {
		t.Fatal(err)
	}
	if tx.WordCount() < 30 {
		t.Fatalf("слов %d", tx.WordCount())
	}
}

func TestParseRejects(t *testing.T) {
	cases := map[string]string{
		"id не по шаблону":         strings.Replace(okText("t100-001"), `"id":"t100-001"`, `"id":"x1"`, 1),
		"id не по уровню":          strings.Replace(okText("t100-001"), `"id":"t100-001"`, `"id":"t300-001"`, 1),
		"уровень вне списка":       strings.Replace(okText("t100-001"), `"band":100`, `"band":150`, 1),
		"мало целевых слов":        strings.Replace(okText("t100-001"), `,{"lemma":"run","pos":"verb"}`, ``, 1),
		"целевое слово один раз":   strings.Replace(okText("t100-001"), `{"lemma":"run","pos":"verb"}`, `{"lemma":"fly","pos":"verb"}`, 1),
		"заглавная в лемме":        strings.Replace(okText("t100-001"), `"lemma":"dog"`, `"lemma":"Dog"`, 1),
		"часть речи вне списка":    strings.Replace(okText("t100-001"), `"pos":"noun"`, `"pos":"pronoun"`, 1),
		"нет перевода":             strings.Replace(okText("t100-001"), `"ru":"а"`, `"ru":""`, 1),
		"нет источника":            strings.Replace(okText("t100-001"), `"sources":["s"]`, `"sources":[]`, 1),
		"лишнее поле":              strings.Replace(okText("t100-001"), `"title":"Т"`, `"title":"Т","extra":1`, 1),
		"слишком короткий":         `{"id":"t100-002","title":"Т","band":100,"targets":[{"lemma":"dog","pos":"noun"},{"lemma":"big","pos":"adjective"},{"lemma":"run","pos":"verb"}],"sentences":[{"en":"A dog.","ru":"а"},{"en":"A big dog.","ru":"б"},{"en":"The dog can run, big dog, dog run big.","ru":"в"}],"sources":["s"]}`,
		"повтор целевого в списке": strings.Replace(okText("t100-001"), `{"lemma":"big","pos":"adjective"}`, `{"lemma":"dog","pos":"noun"}`, 1),
		"хвост после текста":       okText("t100-001") + `{}`,
	}
	for name, data := range cases {
		if _, err := Parse([]byte(data)); err == nil {
			t.Errorf("%s: ошибки нет", name)
		}
	}
}

func TestCheckVocabulary(t *testing.T) {
	tx, err := Parse([]byte(okText("t100-001")))
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{"has": true, "a": true, "the": true, "to": true, "and": true, "likes": true, "lot": true, "man": true, "with": true, "is": true, "here": true}
	for _, w := range []string{"tom"} { // имя из names не проверяется даже если его нет в словаре
		_ = w
	}
	if probs := tx.CheckVocabulary(allowed); len(probs) != 0 {
		t.Fatalf("целевые слова, имена и слова словаря допустимы: %v", probs)
	}
	delete(allowed, "lot")
	probs := tx.CheckVocabulary(allowed)
	if len(probs) != 1 || probs[0].Word != "lot" || probs[0].Sentence != 4 {
		t.Fatalf("слово вне словаря должно называться: %v", probs)
	}
}

func TestContractionsAreCheckedByBase(t *testing.T) {
	for in, want := range map[string]string{"don't": "do", "can't": "can", "won't": "will", "i'm": "i", "it's": "it", "they're": "they", "we'll": "we", "isn't": "is"} {
		if got, ok := contractionBase(in); !ok || got != want {
			t.Errorf("%s → %q (%v), ждали %q", in, got, ok, want)
		}
	}
	if _, ok := contractionBase("dog"); ok {
		t.Error("dog — не сокращение")
	}
}

func TestLoadAllChecksDuplicatesAndBatches(t *testing.T) {
	fsys := fstest.MapFS{"texts/a.json": {Data: []byte("[" + okText("t100-001") + "," + okText("t100-002") + "]")}}
	got, err := LoadAll(fsys)
	if err != nil || len(got) != 2 || got[0].ID != "t100-001" {
		t.Fatalf("%+v %v", got, err)
	}
	fsys["texts/b.json"] = &fstest.MapFile{Data: []byte("[" + okText("t100-001") + "]")}
	if _, err := LoadAll(fsys); err == nil || !strings.Contains(err.Error(), "уже есть") {
		t.Fatalf("повтор id: %v", err)
	}
	if _, err := LoadAll(fstest.MapFS{"texts/c.json": {Data: []byte(`{}`)}}); err == nil {
		t.Fatal("файл — список, а не объект")
	}
}

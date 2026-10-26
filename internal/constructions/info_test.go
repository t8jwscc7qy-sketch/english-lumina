package constructions

import (
	"strings"
	"testing"
	"testing/fstest"
)

const goodInfo = `{"id":"x","title":"T","form":"F","explanation":"Коротко.","exampleEn":"a","exampleRu":"б","sources":["s"]}`

func TestLoadAllValidates(t *testing.T) {
	ok := fstest.MapFS{"constructions/x.json": {Data: []byte(goodInfo)}}
	got, err := LoadAll(ok)
	if err != nil || len(got) != 1 || got["x"].Title != "T" {
		t.Fatalf("%+v %v", got, err)
	}
	long := strings.Repeat("слово ", 61)
	bad := map[string]string{
		"нет источников":    strings.Replace(goodInfo, `"sources":["s"]`, `"sources":[]`, 1),
		"длинное пояснение": strings.Replace(goodInfo, "Коротко.", long, 1),
		"нет примера":       strings.Replace(goodInfo, `"exampleEn":"a"`, `"exampleEn":""`, 1),
		"лишнее поле":       strings.Replace(goodInfo, `"title":"T"`, `"title":"T","extra":1`, 1),
		"не JSON":           `{`,
	}
	for name, data := range bad {
		if _, err := LoadAll(fstest.MapFS{"constructions/x.json": {Data: []byte(data)}}); err == nil {
			t.Errorf("%s: ошибки нет", name)
		}
	}
	dup := fstest.MapFS{"constructions/a.json": {Data: []byte(goodInfo)}, "constructions/b.json": {Data: []byte(goodInfo)}}
	if _, err := LoadAll(dup); err == nil || !strings.Contains(err.Error(), "повторяется") {
		t.Fatalf("повтор id: %v", err)
	}
}

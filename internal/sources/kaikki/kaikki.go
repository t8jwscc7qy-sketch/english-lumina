// Package kaikki описывает источник «Kaikki.org (Wiktextract)»: разобранный английский Викисловарь. Сам файл
// скачивает и отбирает команда cmd/kb-prepare (каркас importer.Run не подходит: данные не грузятся построчно в таблицы
// источника, а сливаются в базу знаний kb_* командой cmd/kb-build).
package kaikki

import "github.com/t8jwscc7qy-sketch/english-lumina/internal/importer"

const (
	// Name — имя источника в data_sources.
	Name = "kaikki-wiktextract"
	// DumpDate — дата дампа Викисловаря, из которого собран файл (версия данных).
	DumpDate = "2026-10-01"
	// URL файла: перезаписывается на сайте примерно раз в неделю и не имеет опубликованной суммы, поэтому сумма
	// закрепляется здесь после первой загрузки; изменившийся файл — ошибка суммы, обновление делается осознанно.
	URL = "https://kaikki.org/dictionary/raw-wiktextract-data.jsonl.gz"
	// FileName — имя файла в кэше (.cache/sources/kaikki/).
	FileName = "raw-wiktextract-data-" + DumpDate + ".jsonl.gz"
	// FileSHA256 — закрепляется после первой загрузки (шаг «живой прогон» плана); пусто — загрузка отказывается.
	FileSHA256 = ""
	// MaxFileSize — потолок размера файла (сейчас ≈ 2,9 ГБ).
	MaxFileSize = 6 << 30

	license     = "CC BY-SA и GFDL (как у Викисловаря; страница загрузок Kaikki лицензию не называет, условия Викисловаря подтвердить перед публикацией данных)"
	attribution = "Wiktionary contributors (en.wiktionary.org), CC BY-SA и GFDL; данные разобраны Wiktextract (Tatu Ylonen), kaikki.org. Ссылка на работу: Tatu Ylonen, Wiktextract: Wiktionary as Machine-Readable Structured Data, LREC 2022, pp. 1317-1325."
)

// Describe возвращает описание источника для data_sources.
func Describe() importer.Info {
	return importer.Info{Name: Name, Version: DumpDate, License: license, URL: URL, Attribution: attribution}
}

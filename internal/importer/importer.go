// Package importer загружает внешние базы знаний (списки уровней, предложения, частоты) в PostgreSQL.
// Каждый источник — отдельный модуль в internal/sources/<имя>, реализующий Source; остальное приложение
// о нём не знает. Подключение нового источника — одна строка регистрации в cmd/import.
package importer

import (
	"context"
	"fmt"
	"io"
	"slices"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

// Info описывает источник: по ней заполняется data_sources и страница «Источники и лицензии».
type Info struct {
	Name        string // короткое имя для команды import и data_sources.name
	Version     string // версия данных; новая версия — новая запись в data_sources
	License     string
	URL         string
	Attribution string // текст, который обязаны показать по лицензии
}

// Source — контракт модуля-источника.
type Source interface {
	Describe() Info
	// Fetch скачивает данные в cacheDir (вне Git), проверяет контрольную сумму и возвращает путь к файлу.
	// Скачанное недоверенно.
	Fetch(ctx context.Context, cacheDir string) (path string, err error)
	// Parse читает файл потоком и отдаёт строки по одной; всё в память не загружает.
	// Тип строки (any) известен только модулю: его Load получает то, что отдал его же Parse.
	Parse(ctx context.Context, r io.Reader, emit func(row any) error) error
	// Load записывает одну строку через q, который работает в транзакции запуска. Каждая строка получает sourceID.
	Load(ctx context.Context, q *db.Queries, sourceID int64, row any) error
}

// Registry хранит доступные источники по имени.
type Registry struct {
	sources map[string]Source
}

func NewRegistry() *Registry {
	return &Registry{sources: map[string]Source{}}
}

// Register добавляет источник. Повторное имя — ошибка программиста, поэтому паника при старте.
func (r *Registry) Register(s Source) {
	name := s.Describe().Name
	if _, dup := r.sources[name]; dup {
		panic(fmt.Sprintf("importer: источник %q уже зарегистрирован", name))
	}
	r.sources[name] = s
}

func (r *Registry) Get(name string) (Source, bool) {
	s, ok := r.sources[name]
	return s, ok
}

// Names возвращает имена источников по алфавиту.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.sources))
	for n := range r.sources {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}

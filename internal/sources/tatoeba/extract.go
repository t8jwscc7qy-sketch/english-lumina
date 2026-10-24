package tatoeba

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// extractMember достаёт из zip один файл с точным именем member и кладёт его в dest.
// Архив недоверенный, поэтому: имя сравнивается целиком (пути из архива не используются вовсе, значит
// `../x` и `/x` никуда не записываются); запись должна быть обычным файлом; читается не больше maxBytes+1
// байт, какой бы размер ни был заявлен в заголовке (так архив-бомба не заполнит диск); запись идёт во
// временный файл рядом и переименовывается в конце, поэтому при любой ошибке dest остаётся как был.
func extractMember(zipPath, member, dest string, maxBytes int64) (err error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("чтение архива: %w", err)
	}
	defer func() { _ = zr.Close() }()

	var found *zip.File
	for _, f := range zr.File {
		if f.Name == member {
			found = f
			break
		}
	}
	if found == nil {
		return fmt.Errorf("в архиве нет файла %s", member)
	}
	if !found.Mode().IsRegular() {
		return fmt.Errorf("%s в архиве не обычный файл", member)
	}
	if found.UncompressedSize64 > uint64(maxBytes) {
		return fmt.Errorf("%s больше лимита распаковки %d байт", member, maxBytes)
	}

	rc, err := found.Open()
	if err != nil {
		return fmt.Errorf("открытие %s в архиве: %w", member, err)
	}
	defer func() { _ = rc.Close() }()

	tmp, err := os.CreateTemp(filepath.Dir(dest), ".extract-*")
	if err != nil {
		return fmt.Errorf("временный файл: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()

	n, err := io.Copy(tmp, io.LimitReader(rc, maxBytes+1))
	if err != nil {
		return fmt.Errorf("распаковка %s: %w", member, err)
	}
	if n > maxBytes {
		err = errors.New(member + " больше лимита распаковки")
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dest)
}

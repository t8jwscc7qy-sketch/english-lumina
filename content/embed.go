// Package content встраивает в сервер тексты шпаргалок (по одному JSON-файлу на шпаргалку).
package content

import "embed"

// Sheets — файлы шпаргалок.
//
//go:embed sheets/*.json
var Sheets embed.FS

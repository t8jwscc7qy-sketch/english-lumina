package config

import "testing"

// По умолчанию сервер слушает только свою машину: вход по HTTP без TLS не должен быть виден в сети.
func TestLoadDefaultListensOnLoopback(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("HTTP_ADDR", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != "127.0.0.1:8080" {
		t.Fatalf("HTTPAddr = %q, ожидали 127.0.0.1:8080", cfg.HTTPAddr)
	}
}

func TestLoadHTTPAddrFromEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("HTTP_ADDR", "127.0.0.1:9000")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != "127.0.0.1:9000" {
		t.Fatalf("HTTPAddr = %q", cfg.HTTPAddr)
	}
}

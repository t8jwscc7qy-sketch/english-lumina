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

func TestAudioNonCommercialDefaultsToOnAndCanBeSwitchedOff(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("AUDIO_NONCOMMERCIAL", "")
	cfg, err := Load()
	if err != nil || !cfg.AudioNonCommercial {
		t.Fatalf("по умолчанию включено: %+v %v", cfg, err)
	}
	t.Setenv("AUDIO_NONCOMMERCIAL", "false")
	if cfg, _ := Load(); cfg.AudioNonCommercial {
		t.Fatal("false выключает озвучку NC/ND")
	}
	t.Setenv("AUDIO_NONCOMMERCIAL", "true")
	if cfg, _ := Load(); !cfg.AudioNonCommercial {
		t.Fatal("true включает")
	}
}

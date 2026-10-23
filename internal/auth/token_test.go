package auth

import (
	"bytes"
	"testing"
)

func TestNewToken(t *testing.T) {
	token, hash, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	if len(token) != 43 {
		t.Errorf("длина токена = %d, ожидали 43", len(token))
	}
	if len(hash) != 32 {
		t.Errorf("длина хеша = %d, ожидали 32", len(hash))
	}
	if !bytes.Equal(HashToken(token), hash) {
		t.Error("HashToken(token) не равен возвращённому хешу")
	}

	other, _, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	if other == token {
		t.Error("два токена совпали")
	}
}

func TestHashTokenDeterministic(t *testing.T) {
	if !bytes.Equal(HashToken("abc"), HashToken("abc")) {
		t.Error("хеш одного токена различается")
	}
	if bytes.Equal(HashToken("abc"), HashToken("abd")) {
		t.Error("разные токены дали один хеш")
	}
}

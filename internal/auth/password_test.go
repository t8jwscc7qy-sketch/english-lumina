package auth

import (
	"strings"
	"testing"
)

func TestHashVerify(t *testing.T) {
	hash, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("хеш не в формате PHC: %q", hash)
	}

	ok, err := VerifyPassword("correct horse battery", hash)
	if err != nil || !ok {
		t.Fatalf("верный пароль: ok=%v err=%v", ok, err)
	}
	ok, err = VerifyPassword("wrong password", hash)
	if err != nil || ok {
		t.Fatalf("неверный пароль: ok=%v err=%v", ok, err)
	}

	other, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if other == hash {
		t.Fatal("два хеша одного пароля совпали: соль не случайная")
	}
}

func TestVerifyMalformed(t *testing.T) {
	for _, phc := range []string{"", "abc", "$argon2id$broken", "$argon2i$v=19$m=1,t=1,p=1$AAAA$AAAA"} {
		ok, err := VerifyPassword("x", phc)
		if err == nil || ok {
			t.Errorf("%q: ok=%v err=%v, ожидали ошибку", phc, ok, err)
		}
	}
}

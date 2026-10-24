package main

import (
	"strings"
	"testing"
)

func TestValidateLogin(t *testing.T) {
	tests := []struct {
		login   string
		wantErr bool
	}{
		{"vlad", false},
		{"a.b-c_9", false},
		{"ab", true},
		{"", true},
		{strings.Repeat("a", 33), true},
		{"Vlad", true}, // заглавные запрещены CHECK в таблице users
		{"vl ad", true},
		{"влад", true},
	}
	for _, tt := range tests {
		if err := validateLogin(tt.login); (err != nil) != tt.wantErr {
			t.Errorf("validateLogin(%q) = %v, wantErr=%v", tt.login, err, tt.wantErr)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"нормальный", "correct horse battery", false},
		{"ровно 12", strings.Repeat("a", 12), false},
		{"короткий", strings.Repeat("a", 11), true},
		{"пустой", "", true},
		{"ровно 256 байт", strings.Repeat("a", 256), false},
		{"257 байт", strings.Repeat("a", 257), true},
	}
	for _, tt := range tests {
		if err := validatePassword(tt.password); (err != nil) != tt.wantErr {
			t.Errorf("%s: validatePassword = %v, wantErr=%v", tt.name, err, tt.wantErr)
		}
	}
}

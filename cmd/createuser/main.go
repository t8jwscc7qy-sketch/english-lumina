// Команда createuser создаёт единственного пользователя сервиса.
// Веб-регистрации нет, поэтому занять сервер из сети нельзя.
package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/term"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/auth"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/config"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

const (
	minPasswordLen   = 12
	maxPasswordBytes = 256 // как в контракте входа
)

// Тот же шаблон, что в CHECK таблицы users.
var loginRe = regexp.MustCompile(`^[a-z0-9_.-]{3,32}$`)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ошибка:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("подключение к базе: %w", err)
	}
	defer pool.Close()
	q := db.New(pool)

	exists, err := q.UserExists(ctx)
	if err != nil {
		return fmt.Errorf("проверка пользователей: %w", err)
	}
	if exists {
		return errors.New("пользователь уже создан (в сервисе он единственный)")
	}

	fmt.Print("Логин (a-z, 0-9, _ . -, 3–32 символа): ")
	login, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return fmt.Errorf("чтение логина: %w", err)
	}
	login = strings.TrimSpace(login)
	if err := validateLogin(login); err != nil {
		return err
	}

	password, err := readPassword("Пароль (от 12 символов): ")
	if err != nil {
		return err
	}
	if err := validatePassword(password); err != nil {
		return err
	}
	again, err := readPassword("Пароль ещё раз: ")
	if err != nil {
		return err
	}
	if !bytes.Equal([]byte(password), []byte(again)) {
		return errors.New("пароли не совпадают")
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	if _, err := q.CreateUser(ctx, db.CreateUserParams{Login: login, PasswordHash: hash}); err != nil {
		return fmt.Errorf("создание пользователя: %w", err)
	}
	fmt.Println("Пользователь создан.")
	return nil
}

// readPassword читает пароль без отображения на экране.
func readPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	b, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("чтение пароля: %w", err)
	}
	return string(b), nil
}

func validateLogin(login string) error {
	if !loginRe.MatchString(login) {
		return errors.New("логин: 3–32 символа, только a-z, 0-9, _ . -")
	}
	return nil
}

func validatePassword(password string) error {
	if len([]rune(password)) < minPasswordLen {
		return fmt.Errorf("пароль короче %d символов", minPasswordLen)
	}
	if len(password) > maxPasswordBytes {
		return fmt.Errorf("пароль длиннее %d байт", maxPasswordBytes)
	}
	return nil
}

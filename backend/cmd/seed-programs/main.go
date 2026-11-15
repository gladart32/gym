// Команда seed-programs загружает готовые программы Gym из манифеста
// data/programs.json в MySQL. Упражнения в манифесте задаются external_id
// каталога (источник exercises-dataset) и резолвятся в exercises.id.
//
// Повторный запуск безопасен: готовая программа (owner_id IS NULL) ищется
// по имени — найденная обновляется целиком (шапка + дни/упражнения/подходы),
// отсутствующая создаётся. Флаг -replace пересоздаёт все готовые программы.
// Всё выполняется в одной транзакции.
//
// Запускается из backend/ (или через make seed-programs из корня репо):
//
//	go run ./cmd/seed-programs            # синхронизация с манифестом
//	go run ./cmd/seed-programs -dry-run   # проверить данные и откатить
//	go run ./cmd/seed-programs -replace   # пересоздать все готовые программы
package main

import (
	"bufio"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

func main() {
	var (
		envFile     = flag.String("env", ".env", "файл с MYSQL_* переменными (необязателен)")
		dsnFlag     = flag.String("dsn", "", "DSN MySQL; по умолчанию собирается из MYSQL_* (env / .env)")
		manifestFl  = flag.String("manifest", "../data/programs.json", "путь к манифесту с готовыми программами")
		replaceFlag = flag.Bool("replace", false, "удалить все готовые программы (owner_id IS NULL) и вставить из манифеста")
		dryRun      = flag.Bool("dry-run", false, "выполнить и откатить транзакцию")
	)
	flag.Parse()
	log.SetFlags(0)

	if err := run(*envFile, *dsnFlag, *manifestFl, *replaceFlag, *dryRun); err != nil {
		log.Fatalf("ошибка: %v", err)
	}
}

func run(envFile, dsn, manifestPath string, replace, dryRun bool) error {
	start := time.Now()

	progs, err := loadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("манифест: %w", err)
	}
	if err := validateManifest(progs); err != nil {
		return err
	}
	log.Printf("манифест: %d программ, %d дней, %d упражнений, %d подходов",
		len(progs), countDays(progs), countExercises(progs), countSets(progs))

	if dsn == "" {
		if dsn, err = dsnFromEnv(envFile); err != nil {
			return err
		}
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("подключение к MySQL: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op после Commit

	s := &seeder{tx: tx}
	st, err := s.seed(ctx, progs, replace)
	if err != nil {
		return err
	}

	if dryRun {
		log.Print("dry-run: транзакция откачена")
	} else if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("программы: создано %d, обновлено %d (дней %d, упражнений %d, подходов %d)",
		st.Created, st.Updated, st.Days, st.Exercises, st.Sets)
	log.Printf("готово за %s", time.Since(start).Round(time.Millisecond))
	return nil
}

// dsnFromEnv собирает DSN из переменных окружения, недостающие берёт из .env.
func dsnFromEnv(envFile string) (string, error) {
	env := map[string]string{}
	if f, err := os.Open(envFile); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if k, v, ok := strings.Cut(line, "="); ok {
				env[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
			}
		}
		f.Close()
		if err := sc.Err(); err != nil {
			return "", err
		}
	}
	get := func(k, def string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		if v := env[k]; v != "" {
			return v
		}
		return def
	}

	cfg := mysql.NewConfig()
	cfg.User = get("MYSQL_USER", "gym")
	cfg.Passwd = get("MYSQL_PASSWORD", "")
	cfg.Net = "tcp"
	cfg.Addr = get("MYSQL_HOST", "127.0.0.1") + ":" + get("MYSQL_PORT", "3306")
	cfg.DBName = get("MYSQL_DATABASE", "gym")
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	cfg.Params = map[string]string{"charset": "utf8mb4"}
	return cfg.FormatDSN(), nil
}

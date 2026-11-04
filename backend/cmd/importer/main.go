// Команда importer загружает каталог упражнений из hasaneyldrm/exercises-dataset
// (make dataset-fetch в корне репо) и русских переводов (data/translations/ru)
// в MySQL Gym.
//
// Повторный запуск безопасен: справочники обновляются, новые упражнения
// добавляются, уже импортированные пропускаются (флаг -update перезаписывает их).
// Всё выполняется в одной транзакции.
//
// Запускается из backend/ (или через make seed-catalog из корня репо):
//
//	go run ./cmd/importer                 # импорт
//	go run ./cmd/importer -dry-run        # проверить данные и откатить
//	go run ./cmd/importer -update         # перезаписать уже импортированные
package main

import (
	"bufio"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

func main() {
	var (
		envFile    = flag.String("env", ".env", "файл с MYSQL_* переменными (необязателен)")
		dsnFlag    = flag.String("dsn", "", "DSN MySQL; по умолчанию собирается из MYSQL_* (env / .env)")
		datasetDir = flag.String("dataset", "../data/exercises-dataset", "путь к клону exercises-dataset")
		trDir      = flag.String("translations", "../data/translations/ru", "путь к русским переводам")
		mediaPref  = flag.String("media-prefix", "", "префикс для путей медиа в БД (например, /media/)")
		update     = flag.Bool("update", false, "перезаписать уже импортированные упражнения (затрёт правки администратора)")
		checkMedia = flag.Bool("check-media", true, "проверять наличие GIF и миниатюр на диске")
		dryRun     = flag.Bool("dry-run", false, "выполнить импорт и откатить транзакцию")
	)
	flag.Parse()
	log.SetFlags(0)

	if err := run(*envFile, *dsnFlag, *datasetDir, *trDir, *mediaPref, *update, *checkMedia, *dryRun); err != nil {
		log.Fatalf("ошибка: %v", err)
	}
}

func run(envFile, dsn, datasetDir, trDir, mediaPrefix string, update, checkMedia, dryRun bool) error {
	start := time.Now()

	src, err := loadSources(datasetDir, trDir)
	if err != nil {
		return fmt.Errorf("загрузка данных: %w", err)
	}
	if err := src.validate(checkMedia); err != nil {
		return err
	}
	log.Printf("данные: %d упражнений, %d частей тела, %d видов оборудования, %d мышц (с синонимами)",
		len(src.Exercises), len(src.BodyParts), len(src.Equipment), len(src.Muscles))

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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("подключение к MySQL: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op после Commit

	im := &importer{tx: tx, src: src, update: update, prefix: mediaPrefix}
	if err := im.importDictionaries(ctx); err != nil {
		return fmt.Errorf("справочники: %w", err)
	}
	st, err := im.importExercises(ctx)
	if err != nil {
		return err
	}

	if dryRun {
		log.Print("dry-run: транзакция откачена")
	} else if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("справочники: частей тела %d, оборудования %d, мышц %d",
		len(im.bodyIDs), len(im.equipID), len(im.muscIDs))
	log.Printf("упражнения: добавлено %d, обновлено %d, пропущено (уже есть) %d",
		st.Inserted, st.Updated, st.Skipped)
	if len(st.ByTracking) > 0 {
		keys := make([]string, 0, len(st.ByTracking))
		for k := range st.ByTracking {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s=%d", k, st.ByTracking[k]))
		}
		log.Printf("tracking_type: %s", strings.Join(parts, ", "))
	}
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

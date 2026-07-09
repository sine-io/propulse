package migrate

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

func Run(_ context.Context, databaseURL string, direction string) error {
	migrationsPath, err := migrationsDir()
	if err != nil {
		return err
	}

	sourceURL := "file://" + migrationsPath
	dbURL := databaseURL
	if dbURL == "" {
		return errors.New("database url is required")
	}

	m, err := migrate.New(sourceURL, dbURL)
	if err != nil {
		return err
	}
	defer m.Close()

	switch direction {
	case "up":
		err = m.Up()
	case "down":
		err = m.Down()
	default:
		return fmt.Errorf("unknown migration direction %q", direction)
	}
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	return err
}

func migrationsDir() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("unable to resolve migrations directory")
	}
	return filepath.Abs(filepath.Join(filepath.Dir(file), "../../../migrations"))
}

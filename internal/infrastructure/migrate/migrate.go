package migrate

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	// Register the PostgreSQL migration driver.
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	migrationfiles "github.com/sine-io/propulse/migrations"
)

func Run(_ context.Context, databaseURL string, direction string) error {
	dbURL := databaseURL
	if dbURL == "" {
		return errors.New("database url is required")
	}

	source, err := iofs.New(migrationfiles.FS, ".")
	if err != nil {
		return err
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, dbURL)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = m.Close()
	}()

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

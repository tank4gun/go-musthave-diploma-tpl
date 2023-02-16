package db

import (
	"fmt"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func RunMigrations(dbURI string) error {
	if dbURI == "" {
		return fmt.Errorf("got empty dbURI")
	}
	m, err := migrate.New("file://internal/db/migrations", dbURI)
	if err != nil {
		fmt.Printf("Got err %s", err.Error())
		return err
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		fmt.Printf("Got err %s", err.Error())
		return err
	}
	return nil
}

package infra

import (
	"context"
	"database/sql"
	"log"

	"backend-challenge-go/config"

	_ "github.com/jackc/pgx/v5/stdlib" // Driver pgx para database/sql
	"go.uber.org/fx"
)

var Module = fx.Options(
	fx.Provide(
		config.LoadConfig,
		NewDatabaseConnection,
	),
)

func NewDatabaseConnection(lc fx.Lifecycle, cfg *config.Config) (*sql.DB, error) {
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Println("🔌 Conectando ao banco de dados PostgreSQL...")
			return db.PingContext(ctx)
		},
		OnStop: func(ctx context.Context) error {
			log.Println("🛑 Fechando conexão com o PostgreSQL...")
			return db.Close()
		},
	})

	return db, nil
}
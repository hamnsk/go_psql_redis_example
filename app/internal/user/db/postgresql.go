package db

import (
	"context"
	"os"
	"redis/internal/user"

	"github.com/jackc/pgx/v4/pgxpool"
)

var _ user.Storage = &db{}

type db struct {
	pool *pgxpool.Pool
}

func NewStorage() (*db, error) {
	pool, err := pgxpool.Connect(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		return nil, err
	}

	return &db{
		pool: pool,
	}, nil
}

func (p *db) Close() {
	p.pool.Close()
}

func (p *db) FindOne(id string) (u user.User, err error) {
	query := `SELECT id, nickname, firstname, lastname, gender, pass, status FROM "users" WHERE id = $1`

	var res user.User

	if err := p.pool.QueryRow(context.Background(), query, id).
		Scan(&res.Id, &res.NickName, &res.FistName, &res.LastName, &res.Gender, &res.Pass, &res.Status); err != nil {
		return user.User{}, err
	}

	return res, nil
}

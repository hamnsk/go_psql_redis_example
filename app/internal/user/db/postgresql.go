package db

import (
	"context"
	"fmt"
	"os"
	"redis/internal/user"
	"redis/pkg/logging"
	"time"

	"github.com/jackc/pgx/v4/log/zapadapter"
	"github.com/jackc/pgx/v4/pgxpool"
)

var _ user.Storage = &db{}

type db struct {
	pool   *pgxpool.Pool
	logger *logging.Logger
}

func NewStorage(appLogger *logging.Logger) (*db, error) {
	config, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		return nil, err
	}

	config.ConnConfig.Logger = zapadapter.NewLogger(appLogger.Logger)
	config.ConnConfig.PreferSimpleProtocol = true

	pool, err := pgxpool.ConnectConfig(context.Background(), config)
	if err != nil {
		return nil, err
	}

	return &db{
		pool:   pool,
		logger: appLogger,
	}, nil
}

func (p *db) Close() {
	p.pool.Close()
}

func (p *db) GetByID(id string) (u user.User, err error) {
	defer trace(*p.logger, id)()
	query := `SELECT id, nickname, firstname, lastname, gender, pass, status FROM "users" WHERE id = $1`

	var res user.User

	conn, err := p.pool.Acquire(context.Background())
	if err != nil {
		return user.User{}, err
	}
	defer conn.Release()

	if err := conn.QueryRow(context.Background(), query, id).
		Scan(&res.Id, &res.NickName, &res.FistName, &res.LastName, &res.Gender, &res.Pass, &res.Status); err != nil {
		return user.User{}, err
	}

	return res, nil
}

func (p *db) FindOneByNickName(nickname string) (u user.User, err error) {
	query := `SELECT id, nickname, firstname, lastname, gender, pass, status FROM "users" WHERE nickname LIKE $1 LIMIT 1`

	var res user.User

	conn, err := p.pool.Acquire(context.Background())
	if err != nil {
		return user.User{}, err
	}
	defer conn.Release()

	if err := conn.QueryRow(context.Background(), query, nickname).
		Scan(&res.Id, &res.NickName, &res.FistName, &res.LastName, &res.Gender, &res.Pass, &res.Status); err != nil {
		return user.User{}, err
	}

	return res, nil
}

func (p *db) GetAll() (users []user.User, err error) {
	query := `SELECT id, nickname, firstname, lastname, gender, pass, status FROM users`

	conn, err := p.pool.Acquire(context.Background())
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	rows, err := conn.Query(context.Background(), query)
	if err != nil {
		return nil, err
	}

	users = make([]user.User, 0)

	for rows.Next() {
		var u user.User
		err = rows.Scan(&u)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}
	return users, nil

}

func (p *db) PingPool(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

func trace(l logging.Logger, id string) func() {
	start := time.Now()
	return func() {
		t := time.Since(start)
		msg := fmt.Sprintf("Time for get user by id=%s from Database is: %s", id, t)
		l.Info(msg, l.Duration("time_duration", t))
	}
}

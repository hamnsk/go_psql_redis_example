package db

import (
	"context"
	"errors"
	"os"
	"redis/internal/user"
	"redis/pkg/logging"
	"time"

	"github.com/jackc/pgx/v4/log/zapadapter"
	"github.com/jackc/pgx/v4/pgxpool"
)

var _ user.Storage = &db{}

const KeepAlivePollPeriod = 3

//const ZapInfoLevel = 0

//const (
//	LogLevelTrace = 6
//	LogLevelDebug = 5
//	LogLevelInfo  = 4
//	LogLevelWarn  = 3
//	LogLevelError = 2
//	LogLevelNone  = 1
//)

type db struct {
	pool   *pgxpool.Pool
	logger *logging.Logger
	config *pgxpool.Config
}

func NewStorage(appLogger *logging.Logger) (user.Storage, error) {
	config := initConfig(appLogger)
	pool, err := dial(context.Background(), config)
	if err != nil {
		return &db{
			pool:   nil,
			logger: appLogger,
			config: config,
		}, err
	}
	return &db{
		pool:   pool,
		logger: appLogger,
		config: config,
	}, nil
}

func initConfig(appLogger *logging.Logger) *pgxpool.Config {
	config, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		return nil
	}
	config.ConnConfig.Logger = zapadapter.NewLogger(appLogger.Logger)
	// TODO: refactor this

	zapLogLevel := appLogger.GetLevel()
	if zapLogLevel == "INFO" {
		config.ConnConfig.LogLevel = 2
	} else {
		config.ConnConfig.LogLevel = 4
	}
	config.ConnConfig.PreferSimpleProtocol = true
	return config
}

func dial(ctx context.Context, config *pgxpool.Config) (*pgxpool.Pool, error) {
	return pgxpool.ConnectConfig(ctx, config)
}

func (p *db) Create(u *user.User) error {
	query := `INSERT INTO "users" (nickname, firstname, lastname, gender, pass, status) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
	conn, err := p.pool.Acquire(context.Background())
	if err != nil {
		return err
	}
	defer conn.Release()

	if err := conn.QueryRow(context.Background(), query, u.NickName, u.FistName, u.LastName, u.Gender, u.Pass, u.Status).Scan(&u.Id); err != nil {
		return err
	}
	return err
}

func (p *db) FindAll(limit, offset int64) (users []user.User, err error) {
	query := `SELECT id, nickname, firstname, lastname, gender, pass, status FROM users WHERE id > $1 LIMIT $2`

	conn, err := p.pool.Acquire(context.Background())
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	rows, err := conn.Query(context.Background(), query, offset, limit)
	if err != nil {
		return nil, err
	}

	users = make([]user.User, 0)

	for rows.Next() {
		var u user.User
		err = rows.Scan(&u.Id, &u.NickName, &u.FistName, &u.LastName, &u.Gender, &u.Pass, &u.Status)
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

func (p *db) FindOne(id string) (u user.User, err error) {
	//defer trace(*p.logger, id)()
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

func (p *db) Update(u *user.User) error {
	query := `UPDATE "users" SET nickname=$1, firstname=$2, lastname=$3, gender=$4, pass=$5, status=$6 WHERE id=$7 RETURNING id`

	conn, err := p.pool.Acquire(context.Background())
	if err != nil {
		return err
	}
	defer conn.Release()

	cmdTag, err := conn.Exec(context.Background(), query, u.NickName, u.FistName, u.LastName, u.Gender, u.Pass, u.Status, u.Id)
	if cmdTag.RowsAffected() == 0 {
		return errors.New("user for update not found")
	}
	return err
}

func (p *db) Delete(id string) error {
	query := `DELETE FROM "users" WHERE id = $1 RETURNING id`

	conn, err := p.pool.Acquire(context.Background())
	if err != nil {
		return err
	}
	defer conn.Release()
	cmdTag, err := conn.Exec(context.Background(), query, id)
	if cmdTag.RowsAffected() == 0 {
		return errors.New("user for delete not found")
	}
	return err
}

func (p *db) Close() {
	p.pool.Close()
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

func (p *db) PingPool(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

func (p *db) KeepAlive() {
	var err error
	for {
		time.Sleep(time.Second * KeepAlivePollPeriod)
		lostConnect := false
		if p.pool == nil {
			lostConnect = true
		} else if err = p.PingPool(context.Background()); err != nil {
			lostConnect = true
		}
		if !lostConnect {
			continue
		}
		p.logger.Info("Reconnect to Postgresql...")
		p.pool, err = dial(context.Background(), p.config)
		if err != nil {
			continue
		}
	}
}

//func trace(l logging.Logger, id string) func() {
//	start := time.Now()
//	return func() {
//		t := time.Since(start)
//		msg := fmt.Sprintf("Time for get user by id=%s from Database is: %s", id, t)
//		l.Info(msg, l.Duration("time_duration", t))
//	}
//}

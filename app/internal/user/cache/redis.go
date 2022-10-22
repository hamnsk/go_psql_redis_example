package cache

import (
	"bytes"
	"context"
	"encoding/gob"
	"github.com/go-redis/redis/v9"
	"os"
	"redis/internal/user"
	"redis/pkg/logging"
	"strconv"
	"time"
)

var _ user.Cache = &cache{}

const KeepAlivePollPeriod = 3

type cache struct {
	client *redis.Client
	logger *logging.Logger
}

func dial() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:        os.Getenv("REDIS"),
		DB:          0,
		DialTimeout: 100 * time.Millisecond,
		ReadTimeout: 100 * time.Millisecond,
	})
}

func New(appLogger *logging.Logger) (*cache, error) {
	client := dial()
	if _, err := client.Ping(context.Background()).Result(); err != nil {
		return &cache{
			client: nil,
			logger: appLogger,
		}, err
	}

	return &cache{
		client: client,
		logger: appLogger,
	}, nil
}

func (c *cache) Get(ctx context.Context, id string) (user.User, error) {
	cmd := c.client.Get(ctx, id)

	cmdb, err := cmd.Bytes()
	if err != nil {
		return user.User{}, err
	}

	b := bytes.NewReader(cmdb)

	var res user.User

	if err := gob.NewDecoder(b).Decode(&res); err != nil {
		return user.User{}, err
	}

	return res, nil
}

func (c *cache) Set(ctx context.Context, u user.User) error {
	var b bytes.Buffer

	if err := gob.NewEncoder(&b).Encode(u); err != nil {
		return err
	}

	return c.client.Set(ctx, strconv.FormatInt(u.Id, 10), b.Bytes(), 25*time.Second).Err()
}

func (c *cache) SetByNickname(ctx context.Context, u user.User) error {
	var b bytes.Buffer

	if err := gob.NewEncoder(&b).Encode(u); err != nil {
		return err
	}

	return c.client.Set(ctx, u.NickName, b.Bytes(), 25*time.Second).Err()
}

func (c *cache) Expire(ctx context.Context, id string) error {
	return c.client.Expire(ctx, id, 25*time.Second).Err()
}

func (c *cache) Del(ctx context.Context, id string) error {
	return c.client.Del(ctx, id).Err()
}

func (c *cache) PingClient(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func (c *cache) Close() error {
	return c.client.Close()
}

func (c *cache) KeepAlive() {
	var err error
	for {
		time.Sleep(time.Second * KeepAlivePollPeriod)
		lostConnect := false
		if c.client == nil {
			lostConnect = true
		} else if err = c.PingClient(context.Background()); err != nil {
			lostConnect = true
		}
		if !lostConnect {
			continue
		}
		c.logger.Info("Reconnect to Redis...")
		c.client = dial()
		if c.client == nil {
			continue
		}
	}
}

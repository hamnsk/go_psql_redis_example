package cache

import (
	"bytes"
	"context"
	"encoding/gob"
	"os"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"

	"redis/internal/user"
)

var _ user.Cache = &cache{}

type cache struct {
	client *redis.Client
}

func New() (*cache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:        os.Getenv("REDIS"),
		DB:          0,
		DialTimeout: 100 * time.Millisecond,
		ReadTimeout: 100 * time.Millisecond,
	})

	if _, err := client.Ping(context.Background()).Result(); err != nil {
		return nil, err
	}

	return &cache{
		client: client,
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

	return c.client.Set(ctx, strconv.FormatInt(u.Id, 10), b.Bytes(), 25 * time.Second).Err()
}

func (c *cache) Expire(ctx context.Context, id string) error {
	return c.client.Expire(ctx, id, 25 * time.Second).Err()
}

func (c *cache) PingClient(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func (c *cache) Close() error {
	return c.client.Close()
}

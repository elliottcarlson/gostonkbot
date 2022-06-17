package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
)

var Redis *RedisClient

func InitRedis(config RedisConfig) {
	if Redis == nil {
		opt, err := redis.ParseURL(config.RedisURL)
		if err != nil {
			panic(err)
		}

		r := redis.NewClient(opt)

		result, err := r.Ping(r.Context()).Result()

		for err != nil || result != "PONG" {
			log.WithFields(log.Fields{
				"url":    config.RedisURL,
				"err":    err,
				"result": result,
			}).Error("Retrying connection to redis.")

			time.Sleep(5 * time.Second)
			result, err = r.Ping(r.Context()).Result()
		}

		Redis = &RedisClient{
			client: r,
			quit:   make(chan struct{}),
			mutex:  &sync.Mutex{},
			prefix: config.Prefix,
		}

		log.Info("Connected to redis.")
	}
}

func (r *RedisClient) Set(userID string, value *User) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	data, _ := json.Marshal(value)

	if _, err := r.client.HSet(r.client.Context(), r.prefix, userID, data).Result(); err != nil {
		return err
	}

	return nil
}

func (r *RedisClient) Delete(userID string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.client.HDel(r.client.Context(), r.prefix, userID)
}

func (r *RedisClient) Get(userID string) (*User, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	raw := r.client.HGet(r.client.Context(), r.prefix, userID).Val()

	var data User
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, err
	}

	return &data, nil

}

func (r *RedisClient) ForEach(callback func(user User)) {
	if result, err := r.client.HGetAll(r.client.Context(), r.prefix).Result(); len(result) > 0 && err == nil {
		for _, raw := range result {
			var data User
			if err := json.Unmarshal([]byte(raw), &data); err == nil {
				callback(data)
			}
		}
	}
}

func (r *RedisClient) GetAllUsers() (users []*User) {
	if result, err := r.client.HGetAll(r.client.Context(), r.prefix).Result(); len(result) > 0 && err == nil {
		for _, raw := range result {
			var data User
			if err := json.Unmarshal([]byte(raw), &data); err == nil {
				users = append(users, &data)
			}
		}
	}

	return users
}

func (r *RedisClient) Ping() error {
	if err := r.client.Ping(context.Background()).Err(); err != nil {
		return fmt.Errorf("unable to check Redis connection: %v", err)
	}

	return nil
}

func (r *RedisClient) Start() {
	if !r.started {
		r.started = true
		for range r.quit {
			close(r.quit)
		}
	}
}

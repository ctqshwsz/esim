package example1

import (
	"context"
	_ "os"

	aa "github.com/jukylin/esim/redis"
	"github.com/gomodule/redigo/redis"
	"github.com/jukylin/esim/tool/ifacer/example/repo"
)

type TestStruct struct{

}


type Close interface {
	Close(string, int) error

	Err() error
}



type Test interface {
	Iface1(func(string) string) (result bool, pool redis.Pool)

	Iface2(ctx context.Context, found *bool) (result bool, err error)

	Iface3() (f func(repo.Repo) string)

	Iface4(map[string]*aa.RedisClient) map[string]string

	Iface5(redisClient *aa.RedisClient) (*aa.RedisClient)

	Iface6(redisClient aa.RedisClient) (aa.RedisClient)

	Iface7(chan<- bool, chan<- aa.RedisClient) <-chan bool

	Iface8(rp repo.Repo) repo.Repo

	Close

	Iface9(TestStruct, []TestStruct, [3]TestStruct)

	Iface10(Close)
}


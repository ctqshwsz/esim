package mysql

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jukylin/esim/config"
	"github.com/jukylin/esim/log"
	"github.com/ory/dockertest/v3"
	dc "github.com/ory/dockertest/v3/docker"
	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

var (
	test1Config = DbConfig{
		Db:      "test_1",
		Dsn:     "root:123456@tcp(localhost:3306)/test_1?charset=utf8&parseTime=True&loc=Local",
		MaxIdle: 10,
		MaxOpen: 100}

	test2Config = DbConfig{
		Db:      "test_2",
		Dsn:     "root:123456@tcp(localhost:3306)/test_1?charset=utf8&parseTime=True&loc=Local",
		MaxIdle: 10,
		MaxOpen: 100}
)

type TestStruct struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

type UserStruct struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

var db *sql.DB
var logger log.Logger

func TestMain(m *testing.M) {
	logger = log.NewLogger(
		log.WithDebug(true),
	)

	pool, err := dockertest.NewPool("")
	if err != nil {
		logger.Fatalf("Could not connect to docker: %s", err)
	}

	opt := &dockertest.RunOptions{
		Repository: "mysql",
		Tag:        "latest",
		Env:        []string{"MYSQL_ROOT_PASSWORD=123456"},
	}

	// pulls an image, creates a container based on it and runs it
	resource, err := pool.RunWithOptions(opt, func(hostConfig *dc.HostConfig) {
		hostConfig.PortBindings = map[dc.Port][]dc.PortBinding{
			"3306/tcp": {{HostIP: "", HostPort: "3306"}},
		}
	})
	if err != nil {
		logger.Fatalf("Could not start resource: %s", err.Error())
	}

	err = resource.Expire(50)
	if err != nil {
		logger.Fatalf(err.Error())
	}

	if err := pool.Retry(func() error {
		var err error
		db, err = sql.Open("mysql",
			"root:123456@tcp(localhost:3306)/mysql?charset=utf8&parseTime=True&loc=Local")
		if err != nil {
			return err
		}
		db.SetMaxOpenConns(100)

		return db.Ping()
	}); err != nil {
		logger.Fatalf("Could not connect to docker: %s", err)
	}

	sqls := []string{
		`create database test_1;`,
		`CREATE TABLE IF NOT EXISTS test_1.test(
		  id int not NULL auto_increment,
		  title VARCHAR(10) not NULL DEFAULT '',
		  PRIMARY KEY (id)
		)engine=innodb;`,
		`create database test_2;`,
		`CREATE TABLE IF NOT EXISTS test_2.user(
		  id int not NULL auto_increment,
		  username VARCHAR(10) not NULL DEFAULT '',
			PRIMARY KEY (id)
		)engine=innodb;`}

	for _, execSQL := range sqls {
		res, err := db.Exec(execSQL)
		if err != nil {
			logger.Errorf(err.Error())
		}
		_, err = res.RowsAffected()
		if err != nil {
			logger.Errorf(err.Error())
		}
	}
	code := m.Run()

	db.Close()
	// You can't defer this because os.Exit doesn't care for defer
	if err := pool.Purge(resource); err != nil {
		logger.Fatalf("Could not purge resource: %s", err)
	}
	os.Exit(code)
}

func TestInitAndSingleInstance(t *testing.T) {
	clientOptions := ClientOptions{}

	client := NewClient(
		clientOptions.WithDbConfig([]DbConfig{test1Config}),
		clientOptions.WithGormConfig(&gorm.Config{
			Logger: logger.(*log.Elogger).Glog(),
		}),
	)
	ctx := context.Background()
	db1 := client.GetCtxDb(ctx, "test_1")
	db1.Exec("use test_1;")
	assert.NotNil(t, db1)

	_, ok := client.gdbs["test_1"]
	assert.True(t, ok)

	assert.Equal(t, client, NewClient())

	client.Close()
}

func TestProxyPatternWithTwoInstance(t *testing.T) {
	clientOnce = sync.Once{}

	clientOptions := ClientOptions{}
	monitorProxyOptions := MonitorProxyOptions{}
	memConfig := config.NewMemConfig()
	memConfig.Set("debug", true)

	client := NewClient(
		clientOptions.WithDbConfig([]DbConfig{test1Config, test2Config}),
		clientOptions.WithConf(memConfig),
		clientOptions.WithGormConfig(&gorm.Config{
			Logger: logger.(*log.Elogger).Glog(),
		}),
		clientOptions.WithProxy(func() interface{} {
			return NewMonitorProxy(
				monitorProxyOptions.WithConf(memConfig),
				monitorProxyOptions.WithLogger(log.NewLogger()))
		}),
	)

	ctx := context.Background()
	db1 := client.GetCtxDb(ctx, "test_1")
	db1.Exec("use test_1;")
	assert.NotNil(t, db1)

	ts := &TestStruct{}
	db1.Table("test").First(ts)
	assert.Nil(t, db1.Error)

	db2 := client.GetCtxDb(ctx, "test_2")
	db2.Exec("use test_2;")
	assert.NotNil(t, db2)

	us := &UserStruct{}
	db2.Table("user").First(us)
	assert.Nil(t, db1.Error)

	client.Close()
}

func TestMulProxyPatternWithOneInstance(t *testing.T) {
	clientOnce = sync.Once{}

	clientOptions := ClientOptions{}
	monitorProxyOptions := MonitorProxyOptions{}
	memConfig := config.NewMemConfig()

	spyProxy1 := newSpyProxy(log.NewLogger(), "spyProxy1")
	spyProxy2 := newSpyProxy(log.NewLogger(), "spyProxy2")
	monitorProxy := NewMonitorProxy(
		monitorProxyOptions.WithConf(memConfig),
		monitorProxyOptions.WithLogger(log.NewLogger()))

	client := NewClient(
		clientOptions.WithDbConfig([]DbConfig{test1Config}),
		clientOptions.WithConf(memConfig),
		clientOptions.WithGormConfig(&gorm.Config{
			Logger: logger.(*log.Elogger).Glog(),
		}),
		clientOptions.WithProxy(
			func() interface{} {
				return spyProxy1
			},
			func() interface{} {
				return spyProxy2
			},
			func() interface{} {
				return monitorProxy
			},
		))

	ctx := context.Background()
	db1 := client.GetCtxDb(ctx, "test_1")
	logger.Infof("db1.ConnPool %p", db1.ConnPool)

	db1.Exec("use test_1;")
	assert.NotNil(t, db1)

	ts := &TestStruct{}
	db1.Table("test").First(ts)
	assert.Nil(t, db1.Error)

	assert.True(t, spyProxy1.QueryWasCalled)
	assert.False(t, spyProxy1.QueryRowWasCalled)
	assert.True(t, spyProxy1.ExecWasCalled)
	assert.False(t, spyProxy1.PrepareWasCalled)

	assert.True(t, spyProxy2.QueryWasCalled)
	assert.False(t, spyProxy2.QueryRowWasCalled)
	assert.True(t, spyProxy2.ExecWasCalled)
	assert.False(t, spyProxy2.PrepareWasCalled)

	client.Close()
}

func TestMulProxyPatternWithTwoInstance(t *testing.T) {
	clientOnce = sync.Once{}

	clientOptions := ClientOptions{}
	memConfig := config.NewMemConfig()

	client := NewClient(
		clientOptions.WithDbConfig([]DbConfig{test1Config, test2Config}),
		clientOptions.WithConf(memConfig),
		clientOptions.WithGormConfig(&gorm.Config{
			Logger: logger.(*log.Elogger).Glog(),
		}),
		clientOptions.WithProxy(
			func() interface{} {
				return newSpyProxy(log.NewLogger(), "spyProxy1")
			},
			func() interface{} {
				return newSpyProxy(log.NewLogger(), "spyProxy2")
			},
			func() interface{} {
				monitorProxyOptions := MonitorProxyOptions{}
				return NewMonitorProxy(
					monitorProxyOptions.WithConf(memConfig),
					monitorProxyOptions.WithLogger(log.NewLogger()))
			},
		),
	)

	ctx := context.Background()
	db1 := client.GetCtxDb(ctx, "test_1")
	db1.Exec("use test_1;")
	assert.NotNil(t, db1)

	ts := &TestStruct{}
	db1.Table("test").First(ts)

	assert.Nil(t, db1.Error)

	db2 := client.GetCtxDb(ctx, "test_2")
	db2.Exec("use test_2;")
	assert.NotNil(t, db2)

	us := &UserStruct{}
	db2.Table("user").First(us)

	assert.Nil(t, db1.Error)

	client.Close()
}

func BenchmarkParallelGetDB(b *testing.B) {
	clientOnce = sync.Once{}

	b.ReportAllocs()
	b.ResetTimer()

	clientOptions := ClientOptions{}
	monitorProxyOptions := MonitorProxyOptions{}
	memConfig := config.NewMemConfig()

	client := NewClient(
		clientOptions.WithDbConfig([]DbConfig{test1Config, test2Config}),
		clientOptions.WithConf(memConfig),
		clientOptions.WithGormConfig(&gorm.Config{
			Logger: logger.(*log.Elogger).Glog(),
		}),
		clientOptions.WithProxy(func() interface{} {
			spyProxy := newSpyProxy(log.NewLogger(), "spyProxy")
			spyProxy.NextProxy(NewMonitorProxy(
				monitorProxyOptions.WithConf(memConfig),
				monitorProxyOptions.WithLogger(log.NewLogger())))

			return spyProxy
		}),
	)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx := context.Background()
			client.GetCtxDb(ctx, "test_1")

			client.GetCtxDb(ctx, "test_2")
		}
	})

	client.Close()

	b.StopTimer()
}

func TestDummyProxy_Exec(t *testing.T) {
	clientOnce = sync.Once{}

	clientOptions := ClientOptions{}
	memConfig := config.NewMemConfig()

	client := NewClient(
		clientOptions.WithDbConfig([]DbConfig{test1Config}),
		clientOptions.WithConf(memConfig),
		clientOptions.WithGormConfig(&gorm.Config{
			Logger: logger.(*log.Elogger).Glog(),
		}),
		clientOptions.WithProxy(
			func() interface{} {
				return newSpyProxy(log.NewLogger(), "spyProxy")
			},
		),
	)
	ctx := context.Background()
	db1 := client.GetCtxDb(ctx, "test_1")
	db1.Exec("use test_1;")
	assert.NotNil(t, db1)

	db1.Table("test").Create(&TestStruct{})

	assert.Equal(t, db1.RowsAffected, int64(0))

	client.Close()
}

func TestClient_GetStats(t *testing.T) {
	clientOnce = sync.Once{}
	clientOptions := ClientOptions{}

	client := NewClient(
		clientOptions.WithDbConfig([]DbConfig{test1Config, test2Config}),
		clientOptions.WithStateTicker(10*time.Millisecond),
		clientOptions.WithGormConfig(&gorm.Config{
			Logger: logger.(*log.Elogger).Glog(),
		}),
		clientOptions.WithProxy(func() interface{} {
			memConfig := config.NewMemConfig()
			monitorProxyOptions := MonitorProxyOptions{}
			return NewMonitorProxy(
				monitorProxyOptions.WithConf(memConfig),
				monitorProxyOptions.WithLogger(log.NewLogger()))
		}),
	)
	ctx := context.Background()
	db1 := client.GetCtxDb(ctx, "test_1")
	db1.Exec("use test_1;")
	assert.NotNil(t, db1)

	time.Sleep(100 * time.Millisecond)

	lab := prometheus.Labels{"db": "test_1", "stats": "max_open_conn"}
	c, _ := mysqlStats.GetMetricWith(lab)
	metric := &io_prometheus_client.Metric{}
	err := c.Write(metric)
	assert.Nil(t, err)

	assert.Equal(t, float64(100), metric.Gauge.GetValue())

	labIdle := prometheus.Labels{"db": "test_1", "stats": "idle"}
	c, _ = mysqlStats.GetMetricWith(labIdle)
	metric = &io_prometheus_client.Metric{}
	err = c.Write(metric)
	assert.Nil(t, err)

	assert.Equal(t, float64(1), metric.Gauge.GetValue())

	client.Close()
}

//nolint:dupl
func TestClient_TxCommit(t *testing.T) {
	clientOnce = sync.Once{}
	clientOptions := ClientOptions{}
	client := NewClient(
		clientOptions.WithDbConfig([]DbConfig{test1Config, test2Config}),
		clientOptions.WithGormConfig(&gorm.Config{
			Logger: logger.(*log.Elogger).Glog(),
		}),
		clientOptions.WithProxy(func() interface{} {
			memConfig := config.NewMemConfig()
			monitorProxyOptions := MonitorProxyOptions{}
			return NewMonitorProxy(
				monitorProxyOptions.WithConf(memConfig),
				monitorProxyOptions.WithLogger(log.NewLogger()))
		}),
	)
	ctx := context.Background()
	db1 := client.GetCtxDb(ctx, "test_1")
	db1.Exec("use test_1;")
	assert.NotNil(t, db1)

	tx := db1.Begin()
	assert.Nil(t, tx.Error)
	tx.Exec("insert into test values (3, 'test')")
	tx.Commit()
	assert.Nil(t, tx.Error)

	test := &TestStruct{}

	db1.Table("test").First(test)

	assert.Equal(t, 1, test.ID)

	client.Close()
}

//nolint:dupl
func TestClient_TxRollBack(t *testing.T) {
	clientOnce = sync.Once{}
	clientOptions := ClientOptions{}
	client := NewClient(
		clientOptions.WithDbConfig([]DbConfig{test1Config, test2Config}),
		clientOptions.WithGormConfig(&gorm.Config{
			Logger: logger.(*log.Elogger).Glog(),
		}),
		clientOptions.WithProxy(func() interface{} {
			memConfig := config.NewMemConfig()
			monitorProxyOptions := MonitorProxyOptions{}
			return NewMonitorProxy(
				monitorProxyOptions.WithConf(memConfig),
				monitorProxyOptions.WithLogger(log.NewLogger(
					log.WithDebug(true),
				)))
		}),
	)
	ctx := context.Background()
	db1 := client.GetCtxDb(ctx, "test_1")
	db1.Exec("use test_1;")
	assert.NotNil(t, db1)

	tx := db1.Begin()
	assert.Nil(t, tx.Error)
	tx.Exec("insert into test values (100, 'test')")
	tx.Rollback()

	assert.Nil(t, tx.Error)

	ts := TestStruct{}
	db1.Table("test").Where("id = 100").First(&ts)
	assert.Equal(t, 0, ts.ID)

	client.Close()
}

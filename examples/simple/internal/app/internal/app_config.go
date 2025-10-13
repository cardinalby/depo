package internal

import "time"

type AppConfig interface {
	GetDbFilepath() string
	GetDbName() string
	GetLogFilepath() string
	GetHttpSrvAddr() string
	GetOpTimeout() time.Duration
}

type appConfig struct {
	DbFilepath  string        `env:"DB_FILEPATH" envDefault:"db.sqlite"`
	DbName      string        `env:"DB_NAME" envDefault:"simple_app"`
	LogFilepath string        `env:"LOG_FILEPATH" envDefault:"simple_app.log"`
	HttpSrvAddr string        `env:"HTTP_SRV_ADDR" envDefault:"localhost:8080"`
	OpTimeout   time.Duration `env:"OP_TIMEOUT" envDefault:"10s"`
}

func (a appConfig) GetDbFilepath() string {
	return a.DbFilepath
}

func (a appConfig) GetDbName() string {
	return a.DbName
}

func (a appConfig) GetLogFilepath() string {
	return a.LogFilepath
}

func (a appConfig) GetHttpSrvAddr() string {
	return a.HttpSrvAddr
}

func (a appConfig) GetOpTimeout() time.Duration {
	return a.OpTimeout
}

package main

import (
	"fmt"

	"github.com/ilyakaznacheev/cleanenv"
)

type WordsConfig struct {
	Port string `yaml:"port" env:"WORDS_GRPC_PORT" env-default:"8080"`
}

func ParseServerConfig(configPath string, addrFlag string) (string, string, error) {
	var cfg WordsConfig

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		return "", "", err
	}

	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return "", "", err
	}

	port := cfg.Port

	if addrFlag == "" {
		addrFlag = fmt.Sprintf(":%s", port)
	}

	return addrFlag, port, nil
}

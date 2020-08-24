package crawler

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type Config struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
}

var Configuration Config

func init() {
	cfg, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		panic(err.Error())
	}
	yaml.Unmarshal(cfg, &Configuration)
	fmt.Println(Configuration)
}

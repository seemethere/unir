package config

import (
	"gopkg.in/yaml.v2"
)

type UnirConfig struct {
	Whitelist       []string `yaml:"whitelist"`
	ApprovalsNeeded int      `yaml:"approvals_needed"`
}

func ReadConfig(input []byte) (UnirConfig, error) {
	conf := UnirConfig{}
	err := yaml.Unmarshal(input, &conf)
	return conf, err
}

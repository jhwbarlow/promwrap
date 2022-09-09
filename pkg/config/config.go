package config

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Both   []*CounterConfig `yaml:"both"`
	Stdout []*CounterConfig `yaml:"stdout"`
	Stderr []*CounterConfig `yaml:"stderr"`
}

type CounterConfig struct {
	CounterName string `yaml:"counterName"`
	Help        string `yaml:"help"`
	Regex       string `yaml:"regex"`
}

type ConfigSourcer interface {
	Config() (*Config, error)
}

type YAMLFileConfigSourcer struct {
	FilePath string
}

func NewYAMLFileConfigSourcer(filePath string) *YAMLFileConfigSourcer {
	return &YAMLFileConfigSourcer{filePath}
}

func (s *YAMLFileConfigSourcer) Config() (*Config, error) {
	bytes, err := ioutil.ReadFile(s.FilePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %q: %w", s.FilePath, err)
	}

	config := new(Config)
	if err := yaml.Unmarshal(bytes, config); err != nil {
		return nil, fmt.Errorf("unmarshaling YAML from file %q: %w", s.FilePath, err)
	}

	return config, nil
}

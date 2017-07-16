package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"
)

type Config struct {
	Addr                 string
	EvernoteClientKey    string
	EvernoteClientSecret string
	EvernoteAccessToken  string

	GoogleAPIClientID     string
	GoogleAPIClientSecret string
	GoogleAPIAccessToken  string
	GoogleAPIRefreshToken string
	GoogleAPIExpiryDate   time.Time

	filepath string `json:"-"`
}

func (c *Config) LoadFromFile(path string) error {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(file, c); err != nil {
		return err
	}
	c.filepath = path
	return nil
}

func (c Config) Save() error {
	if len(c.filepath) <= 0 {
		return fmt.Errorf("No filepath, use SaveAs instead")
	}

	return c.SaveAs(c.filepath)
}

func (c Config) SaveAs(path string) error {
	b, _ := json.MarshalIndent(&c, "", "	")
	return ioutil.WriteFile(path, b, 0644)
}

const (
	configPath = "./config.json"
)

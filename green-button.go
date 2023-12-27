package main

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type alectra struct {
	UserID        string
	Password      string
	UrlAlectra    string
	InfluxAddress string
	InfluxPass    string
}

var alectraConfig alectra

func main() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/green-button/")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()
	viper.SetEnvPrefix("GREEN")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error config file: %w", err))
	}

	err = viper.UnmarshalKey("alectra", &alectraConfig)
	if err != nil {
		panic(fmt.Errorf("fatal error config file: %w", err))
	}

	for {
		_, err = alectraScrape()
		if err != nil {
			fmt.Printf("Alectra failed: %s", err)
		}
		time.Sleep(6 * time.Hour)
	}
}

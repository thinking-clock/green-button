package main

import (
	"crypto/tls"
	"crypto/x509"
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
	RootCAs       string
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

	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM([]byte(alectraConfig.RootCAs)); !ok {
		panic(fmt.Errorf("fatal error loading root certificates"))
	}

	tlsConfig := &tls.Config{
		RootCAs: pool,
	}

	// defaultTransport := http.DefaultTransport.(*http.Transport)
	// defaultTransport.TLSClientConfig = newTlsConfig

	for {
		_, err = alectraScrape(tlsConfig)
		if err != nil {
			fmt.Printf("Alectra failed: %s", err)
		}
		time.Sleep(6 * time.Hour)
	}
}

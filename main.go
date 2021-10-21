package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/j18e/elvanto-overview/pkg/repositories"
	"github.com/j18e/elvanto-overview/pkg/serving"
	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	ClientID     string `required:"true" envconfig:"CLIENT_ID"`
	ClientSecret string `required:"true" envconfig:"CLIENT_SECRET"`
	RedirectURI  string `required:"true" envconfig:"REDIRECT_URI"`
	Domain       string `required:"true" envconfig:"DOMAIN"`
	DataFile     string `required:"true" envconfig:"DATA_FILE"`
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	var conf Config
	if err := envconfig.Process("", &conf); err != nil {
		return err
	}

	repo, err := repositories.NewRepository(conf.DataFile)
	if err != nil {
		return err
	}

	srv := &serving.Server{
		ClientID:     conf.ClientID,
		ClientSecret: conf.ClientSecret,
		RedirectURI:  conf.RedirectURI,
		Domain:       conf.Domain,
		HTTPCli:      &http.Client{Timeout: time.Second * 10},
		Repository:   repo,
	}

	r := gin.Default()
	r.LoadHTMLGlob("template.html")
	r.GET("/", srv.HandleOverview)
	r.GET("/login", srv.HandleLogin)
	r.GET("/login/complete", srv.HandleCompleteLogin)
	listenAddr := ":3000"
	log.Infof("listening on %s", listenAddr)
	return r.Run(listenAddr)
}

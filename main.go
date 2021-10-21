package main

import (
	"flag"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"

	"github.com/j18e/elvanto-overview/pkg/middleware"
	"github.com/j18e/elvanto-overview/pkg/repositories"
	"github.com/j18e/elvanto-overview/pkg/serving"
)

const listenAddr = ":3000"

type Config struct {
	ClientID      string `required:"true" envconfig:"CLIENT_ID"`
	ClientSecret  string `required:"true" envconfig:"CLIENT_SECRET"`
	RedirectURI   string `required:"true" envconfig:"REDIRECT_URI"`
	Domain        string `required:"true" envconfig:"DOMAIN"`
	DataFile      string `required:"true" envconfig:"DATA_FILE"`
	ElvantoDomain string `required:"true" envconfig:"ELVANTO_DOMAIN"`
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	dryRun := flag.Bool("dry-run", false, "just load example data and skip authentication")
	flag.Parse()

	if *dryRun {
		elvantoDomain := os.Getenv("ELVANTO_DOMAIN")
		if elvantoDomain == "" {
			log.Warn("elvanto domain not set")
		}
		r := gin.Default()
		r.LoadHTMLGlob("template.html")
		r.GET("/", serving.DryRunHandler("example-data.json", elvantoDomain))
		return r.Run(listenAddr)
	}

	var conf Config
	if err := envconfig.Process("", &conf); err != nil {
		return err
	}

	repo, err := repositories.NewRepository(conf.DataFile)
	if err != nil {
		return err
	}

	srv := &serving.Server{
		ClientID:      conf.ClientID,
		ClientSecret:  conf.ClientSecret,
		RedirectURI:   conf.RedirectURI,
		Domain:        conf.Domain,
		ElvantoDomain: conf.ElvantoDomain,
		HTTPCli:       &http.Client{Timeout: time.Second * 10},
		Repository:    repo,
	}

	r := gin.Default()
	r.LoadHTMLGlob("template.html")

	r.GET("/", middleware.SetTokens(repo),
		middleware.RequireTokens(),
		middleware.RefreshTokens(srv.HTTPCli, repo),
		srv.HandleOverview,
	)
	r.GET("/login", srv.HandleLogin)
	r.GET("/login/complete", srv.HandleCompleteLogin)
	log.Infof("listening on %s", listenAddr)
	return r.Run(listenAddr)
}

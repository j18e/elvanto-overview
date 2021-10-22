package main

import (
	"flag"
	"net/http"
	"os"
	"time"

	"github.com/alexedwards/scs/boltstore"
	"github.com/alexedwards/scs/v2"
	"github.com/gin-gonic/gin"
	"github.com/kelseyhightower/envconfig"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"go.etcd.io/bbolt"

	"github.com/j18e/elvanto-overview/pkg/middleware"
	"github.com/j18e/elvanto-overview/pkg/serving"
)

const listenAddr = ":3000"

type Config struct {
	ClientID      string `required:"true" envconfig:"CLIENT_ID"`
	ClientSecret  string `required:"true" envconfig:"CLIENT_SECRET"`
	RedirectURI   string `required:"true" envconfig:"REDIRECT_URI"`
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
	logLevel := flag.String("log-level", "info", "level to log on")
	flag.Parse()

	lvl, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		return err
	}
	logrus.SetLevel(lvl)

	if *dryRun {
		return runDry()
	}

	var conf Config
	if err := envconfig.Process("", &conf); err != nil {
		return err
	}

	db, err := bbolt.Open(conf.DataFile, 0600, nil)
	if err != nil {
		return err
	}
	defer db.Close()

	sm := scs.New()
	sm.Store = boltstore.NewWithCleanupInterval(db, time.Hour)
	sm.Lifetime = time.Hour * 24 * 30

	httpCli := &http.Client{Timeout: time.Second * 10}
	mw := middleware.MW{HTTPCli: httpCli, SM: sm}

	srv := &serving.Server{
		ClientID:      conf.ClientID,
		ClientSecret:  conf.ClientSecret,
		RedirectURI:   conf.RedirectURI,
		ElvantoDomain: conf.ElvantoDomain,
		HTTPCli:       httpCli,
		MW:            mw,
	}

	r := gin.Default()
	r.LoadHTMLGlob("template.html")

	r.GET("/", mw.RequireTokens, srv.HandleOverview)
	r.GET("/login", srv.HandleLogin)
	r.GET("/login/complete", srv.HandleCompleteLogin)
	log.Infof("listening on %s", listenAddr)
	return http.ListenAndServe(listenAddr, sm.LoadAndSave(r))
}

func runDry() error {
	elvantoDomain := os.Getenv("ELVANTO_DOMAIN")
	if elvantoDomain == "" {
		log.Warn("elvanto domain not set")
	}
	r := gin.Default()
	r.LoadHTMLGlob("template.html")
	r.GET("/", serving.DryRunHandler("example-data.json", elvantoDomain))
	return r.Run(listenAddr)
}

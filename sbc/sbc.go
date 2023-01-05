package sbc

import (
	"context"
	"fmt"
	"os"

	"github.com/ZeljkoBenovic/tsbc/db"
	"github.com/docker/docker/client"
	"github.com/hashicorp/go-hclog"
)

type Config struct {
	DomainName, Port string
	LogLevel         string
}

type ISBC interface {
	Run()
}

type sbc struct {
	config   Config
	ctx      context.Context
	dockerCl *client.Client
	logger   hclog.Logger
	db       db.IDB
	sbcData  db.Sbc
}

func NewSBC(sbcConfig Config) (ISBC, error) {
	// TODO: logger configurability options
	// create new logger instance
	lg := hclog.New(&hclog.LoggerOptions{
		Name:                 "sbc",
		Level:                hclog.LevelFromString(sbcConfig.LogLevel),
		Color:                hclog.AutoColor,
		ColorHeaderAndFields: true,
	})

	lg.Debug("Creating new docker client instance")

	// create new docker client instance
	dCl, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		lg.Error("Could not instantiate new docker client with opts", "err", err)

		return nil, fmt.Errorf("could not create new docker client instance err=%w", err)
	}

	lg.Debug("New docker client instance created")

	// get new database instance
	dbInst, err := db.NewDB(lg)
	if err != nil {
		lg.Error("Could not instantiate new database instance", "err", err)

		return nil, err
	}

	// return sbc instance
	return &sbc{
		config:   sbcConfig,
		ctx:      context.Background(),
		logger:   lg,
		dockerCl: dCl,
		db:       dbInst,
	}, nil
}

func (s *sbc) close() {
	if err := s.dockerCl.Close(); err != nil {
		s.logger.Error("Could not close docker client", "err", err)
	}

	if err := s.db.Close(); err != nil {
		s.logger.Error("Could not close database client", "err", err)
	}
}

func (s *sbc) Run() {
	// close connections after this function finishes
	defer s.close()

	if err := s.db.CreateFreshDB(); err != nil {
		s.logger.Error("Could not create fresh DB", "err", err)
		os.Exit(1)
	}

	// save sbc configuration information
	sbcId, err := s.db.SaveSBCInformation()
	if err != nil {
		s.logger.Error("Could not save SBC information", "err", err)
		os.Exit(1)
	}

	// get data from database
	s.sbcData, err = s.db.GetSBCParameters(sbcId)
	if err != nil {
		s.logger.Error("Could not get SBC parameters", "err", err)
		os.Exit(1)
	}

	// TODO: implement and handle Lets Encrypt container

	// create and run containers infrastructure
	if err = s.createAndRunSbcInfra(); err != nil {
		s.logger.Error("Could not create SBC infrastructure", "err", err)
		// TODO: add database revert if deployment fails
		os.Exit(1)
	}
}

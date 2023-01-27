package sbc

import (
	"context"
	"fmt"
	"os"

	"github.com/ZeljkoBenovic/tsbc/cmd/helpers/flagnames"
	"github.com/ZeljkoBenovic/tsbc/db"
	"github.com/ZeljkoBenovic/tsbc/sbc/types"
	"github.com/docker/docker/client"
	"github.com/hashicorp/go-hclog"
	"github.com/spf13/viper"
)

type ISBC interface {
	Run()
	Restart(fqdnName string) error
	Recreate(fqdnName string) error
	Destroy(fqdnName string) error
	DestroyLetsEncryptNode() error
	List() ([]types.Sbc, error)

	Close()
}

type sbc struct {
	ctx           context.Context
	dockerCl      *client.Client
	logger        hclog.Logger
	db            db.IDB
	sbcData       types.Sbc
	dockerLogFile *os.File
}

func NewSBC() (ISBC, error) {
	// create sbc instance
	sbcInst := &sbc{
		ctx:     context.Background(),
		sbcData: types.Sbc{},
	}

	// create folders and file paths
	if err := sbcInst.setFilePaths(); err != nil {
		return nil, fmt.Errorf("could not set file paths: %w", err)
	}

	var err error

	// create docker log file
	sbcInst.dockerLogFile, err = os.OpenFile(
		sbcInst.sbcData.DockerLogFileLocation,
		os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("could not create docker log file: %w", err)
	}

	// create new logger instance
	lg := hclog.New(&hclog.LoggerOptions{
		Name:                 "sbc",
		Level:                hclog.LevelFromString(viper.GetString(flagnames.LogLevel)),
		Output:               sbcInst.setLogOutput(),
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

	// create new database instance
	dbInst, err := db.NewDB(lg, sbcInst.sbcData.SQLiteFileLocation)
	if err != nil {
		lg.Error("Could not instantiate new database instance", "err", err)

		return nil, err
	}

	// set all instances to main sbc instance
	sbcInst.db = dbInst
	sbcInst.dockerCl = dCl
	sbcInst.logger = lg

	// return sbc instance
	return sbcInst, nil
}

func (s *sbc) Close() {
	if err := s.dockerCl.Close(); err != nil {
		s.logger.Error("Could not close docker client", "err", err)
	}

	if err := s.db.Close(); err != nil {
		s.logger.Error("Could not close database client", "err", err)
	}

	if err := s.dockerLogFile.Close(); err != nil {
		s.logger.Error("Could not close docker log file handle", "err", err)
	}
}

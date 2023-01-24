package sbc

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ZeljkoBenovic/tsbc/cmd/helpers/flagnames"
	"github.com/ZeljkoBenovic/tsbc/db"
	"github.com/ZeljkoBenovic/tsbc/sbc/types"
	"github.com/docker/docker/client"
	"github.com/hashicorp/go-hclog"
	"github.com/spf13/viper"
)

type ISBC interface {
	Run()
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
	sbcInst.dockerLogFile, err = os.OpenFile(sbcInst.sbcData.DockerLogFileLocation, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
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

func (s *sbc) close() {
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

func (s *sbc) Run() {
	// close connections after this function finishes
	defer s.close()

	// create new schema if db is empty
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

	// create and run lets encrypt node
	if err = s.handleTLSCertificates(); err != nil {
		s.logger.Error("Could not handle TLS certificate", "err", err)
		// if tls deployment fails, cleanup the database
		s.db.RevertLastInsert()
		os.Exit(1)
	}

	// create and run containers infrastructure
	if err = s.createAndRunSbcInfra(); err != nil {
		s.logger.Error("Could not create SBC infrastructure", "err", err)
		// if docker deployment fails, cleanup the database
		s.db.RevertLastInsert()
		os.Exit(1)
	}
}

func (s *sbc) setFilePaths() error {
	var err error

	// set file paths from flag defaults
	s.sbcData.LogFileLocation = viper.GetString(flagnames.LogFileLocation)
	s.sbcData.DockerLogFileLocation = viper.GetString(flagnames.DockerLogFileLocation)
	s.sbcData.SQLiteFileLocation = viper.GetString(flagnames.DBFileLocation)

	// set default location for database file
	if s.sbcData.SQLiteFileLocation == "" {
		s.sbcData.SQLiteFileLocation = db.DefaultDBLocation()
	}

	// create docker log directory
	if err = os.MkdirAll(filepath.Dir(s.sbcData.DockerLogFileLocation), 755); err != nil && !os.IsExist(err) {
		return fmt.Errorf("could not create docker log directory: %w", err)
	}

	// create database directory
	if err = os.MkdirAll(filepath.Dir(s.sbcData.SQLiteFileLocation), 755); err != nil && !os.IsExist(err) {
		return fmt.Errorf("could not create .tsbc directory")
	}

	return nil
}

func (s *sbc) setLogOutput() io.Writer {
	var (
		logOutput io.Writer
		err       error
	)

	// logOutput set to nil will default to console logging
	switch s.sbcData.LogFileLocation {
	case "":
		logOutput = nil
	default:
		// create log directory
		if err = os.MkdirAll(filepath.Dir(s.sbcData.LogFileLocation), 755); err != nil && !os.IsExist(err) {
			s.logger.Error("Could not create log directory, switching to console output", "err", err)
			break
		}

		// create log file
		logOutput, err = os.OpenFile(s.sbcData.LogFileLocation, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			s.logger.Error("Could not create log file", "err", err)
			logOutput = nil
			break
		}
	}

	return logOutput
}

package sbc

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ZeljkoBenovic/tsbc/cmd/helpers/flagnames"
	"github.com/ZeljkoBenovic/tsbc/db"
	"github.com/spf13/viper"
)

func (s *sbc) Run() {
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

package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"

	"github.com/ZeljkoBenovic/tsbc/cmd/helpers/flagnames"
	"github.com/hashicorp/go-hclog"
	"github.com/spf13/viper"
)

func (d *db) getSingleRecordAndIncreaseByValue(increaseValue int, columnName, tableName string) string {
	// get the latest row with the requested column name
	stmt, err := d.db.Prepare(fmt.Sprintf("SELECT %s FROM %s ORDER BY id DESC LIMIT 1", columnName, tableName))
	if err != nil {
		d.log.Error("Could not prepare select statement", "err", err)
		os.Exit(1)
	}

	row, err := stmt.Query()
	if err != nil {
		d.log.Error("Could not run select statement query", "err", err)
	}

	var colValue string

	for row.Next() {
		if err := row.Scan(&colValue); err != nil {
			d.log.Error("Could not get row data from table", "err", err)
			os.Exit(1)
		}
	}

	_ = row.Close()

	// convert string to int
	colInt, err := strconv.Atoi(colValue)
	if err != nil {
		d.log.Error("Could not convert string to int", "err", err)
		os.Exit(1)
	}

	// increase by one and return as string
	return strconv.Itoa(colInt + increaseValue)
}

func checkForRequiredFlags() error {
	// pbx ip can not be undefined
	if viper.GetString(flagnames.KamailioPbxIp) == "" {
		return ErrPbxIpNotDefined
	}

	// sbc name can not be undefined
	if viper.GetString(flagnames.SbcFqdn) == "" {
		return ErrSbcFqdnNotDefined
	}

	// TODO: add checks for valid IP address format
	// public ip address must be defined
	if viper.GetString(flagnames.RtpPublicIp) == "" {
		return ErrRtpEnginePublicIPNotDefined
	}

	return nil
}

func (d *db) checkIfTableExists(tableName string) (bool, error) {
	stmt, err := d.db.Prepare("SELECT name FROM sqlite_master WHERE type='table' AND name=?")
	if err != nil {
		return false, fmt.Errorf("could not prepare statement err=%w", err)
	}

	res, err := stmt.Query(tableName)
	if err != nil {
		return false, fmt.Errorf("could not run query err=%w", err)
	}

	defer res.Close()

	if res.Next() {
		return true, nil
	}

	return false, nil
}

func (d *db) deleteRowWithID(tableName string, insertID int64) {
	stmt, err := d.db.Prepare(fmt.Sprintf("DELETE FROM %s WHERE id = ?", tableName))
	if err != nil {
		d.log.Error("Could not prepare delete statement", "table", tableName, "err", err)
		return
	}

	res, err := stmt.Exec(insertID)
	if err != nil {
		d.log.Error("Could not execute delete statement", "table", tableName, "err", err)
		return
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		d.log.Error("Could not get affected rows", "table", tableName, "err", err)
		return
	}

	d.log.Info("Record deleted successfully", "table", tableName, "deleted_rows", rowsAffected)
}

// insertOrUpdateContainerID checks if the rowID already exists in the database and prepares and executes the statement.
// Needed to be done in order to handle LetsEncrypt node database record after it has been deleted.
func (d *db) insertOrUpdateContainerID(rowID int64, tableName, containerID string) (sql.Result, error) {
	var (
		tableId = new(int64)
		stmt    = new(sql.Stmt)
		result  sql.Result
		err     error
	)
	// check if this rowID exists in the database
	err = d.db.QueryRowContext(
		context.Background(),
		fmt.Sprintf("SELECT id FROM %s WHERE id = %d", tableName, rowID)).
		Scan(tableId)

	switch {
	case err == sql.ErrNoRows:
		stmt, err = d.db.Prepare(fmt.Sprintf("INSERT INTO %s (container_id) VALUES(?)", tableName))
		if err != nil {
			return nil, err
		}

		result, err = stmt.Exec(containerID)

	case err != nil:
		result = nil
		err = fmt.Errorf("could not run InsertOrUpdate: %w", err)

	default:
		stmt, err = d.db.Prepare(fmt.Sprintf("UPDATE %s SET container_id = ? WHERE id = ?", tableName))
		if err != nil {
			return nil, err
		}

		result, err = stmt.Exec(containerID, rowID)
	}

	return result, err
}

func DefaultDBLocation() string {
	logger := hclog.New(hclog.DefaultOptions)
	// get user home dir
	homeDir, err := os.UserHomeDir()
	if err != nil {
		logger.Error("Could not get user home directory, setting sbc.db to local folder", "err", err)
		return "./tsbc/sbc.db"
	}

	// return default DB location
	return homeDir + "/.tsbc/sbc.db"
}

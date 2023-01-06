package db

import (
	"fmt"
	"os"
	"strconv"

	"github.com/ZeljkoBenovic/tsbc/cmd/flagnames"
	"github.com/spf13/viper"
)

func (d *db) getSingleRecordAndIncreaseByValue(increaseValue int, columnName, tableName string) string {
	// get the latest row with the requested column name
	stmt, err := d.db.Prepare(fmt.Sprintf("SELECT %s FROM %s ORDER BY id DESC LIMIT 1;", columnName, tableName))
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

	d.log.Info("Revert completed successfully", "table", tableName, "deleted_rows", rowsAffected)
}

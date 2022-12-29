package db

import (
	"fmt"
	"os"
	"strconv"
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

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ZeljkoBenovic/tsbc/cmd/helpers/flagnames"
	"github.com/ZeljkoBenovic/tsbc/sbc/types"
	"github.com/hashicorp/go-hclog"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/viper"
)

type IDB interface {
	Close() error
	CreateFreshDB() error

	SaveSBCInformation() (int64, error)
	SaveContainerID(rowID int64, tableName, id string) error

	GetSBCParameters(sbcId int64) (types.Sbc, error)
	GetSBCIdFromFqdn(sbcFqdn string) int64
	GetKamailioInsertID(sbcFqdn string) int64
	GetRTPEngineInsertID(sbcFqdn string) int64
	GetContainerIDsFromSbcFqdn(sbcFqdn string) []string
	GetAllFqdnNames() ([]string, error)
	GetLetsEncryptNodeID() (string, error)

	RevertLastInsert()
	RemoveSbcInfo(sbcFqdn string) error
	RemoveLetsEncryptInfo(nodeId string) error
}

var (
	ErrPbxIpNotDefined             = errors.New("pbx ip address not defined")
	ErrSbcFqdnNotDefined           = errors.New("sbc fqdn not defined")
	ErrRtpEnginePublicIPNotDefined = errors.New("rtp engine public ip address not defined")
)

type db struct {
	db  *sql.DB
	log hclog.Logger

	insertID
}

type insertID struct {
	kamailio, rtpEngine, sbcInstance int64
}

func NewDB(logger hclog.Logger, dbLocation string) (IDB, error) {
	var err error
	dbInstance := &db{
		log: logger.Named("db"),
	}

	dbInstance.log.Debug("Creating new SQLite instance")

	dbInstance.db, err = sql.Open("sqlite3", dbLocation)
	if err != nil {
		dbInstance.log.Error("Could not open db connection", "data_source", "sbc.db", "err", err)

		return nil, err
	}

	dbInstance.log.Debug("SQLite instance created")

	return dbInstance, nil
}

func (d *db) Close() error {
	if err := d.db.Close(); err != nil {
		return err
	}

	return nil
}

func (d *db) GetKamailioInsertID(sbcFqdn string) int64 {
	var id int64 = 0

	if err := d.db.QueryRowContext(
		context.Background(),
		"SELECT kamailio_id FROM sbc_info WHERE fqdn=?", sbcFqdn).
		Scan(&id); err != nil {
		d.log.Error("Could not get kamailio_id", "err", err)
	}

	if id != 0 {
		d.insertID.kamailio = id
	}

	return d.insertID.kamailio
}

func (d *db) GetRTPEngineInsertID(sbcFqdn string) int64 {
	var id int64 = 0

	if err := d.db.QueryRowContext(
		context.Background(),
		"SELECT rtp_engine_id FROM sbc_info WHERE fqdn=?", sbcFqdn).
		Scan(&id); err != nil {
		d.log.Error("Could not get rtp_engine_id", "err", err)
	}

	if id != 0 {
		d.insertID.rtpEngine = id
	}

	return d.insertID.rtpEngine
}

func (d *db) GetAllFqdnNames() ([]string, error) {
	rows, err := d.db.QueryContext(context.Background(), "SELECT fqdn from sbc_info")
	if err != nil {
		return nil, fmt.Errorf("could not get fqdns from database: %w", err)
	}

	resp := make([]string, 0)
	fqdn := ""

	for rows.Next() {
		if err = rows.Scan(&fqdn); err != nil {
			d.log.Error("Could not scan fqdn from database", "err", err)
		}

		resp = append(resp, fqdn)
	}

	return resp, nil
}

func (d *db) GetLetsEncryptNodeID() (string, error) {
	var nodeID = new(string)

	// there is always only one letsencrypt node
	err := d.db.QueryRowContext(
		context.Background(),
		"SELECT container_id FROM letsencrypt").
		Scan(nodeID)

	switch {
	case err == sql.ErrNoRows:
		d.log.Debug("No rows in letsencrypt container_id")
	case err != nil:
		return "", err
	}

	return *nodeID, nil
}

func (d *db) RemoveLetsEncryptInfo(nodeId string) error {
	stmt, err := d.db.Prepare("DELETE FROM letsencrypt WHERE container_id = ?")
	if err != nil {
		return fmt.Errorf("could not prepare delete letsencrypt node: %w", err)
	}

	_, err = stmt.Exec(nodeId)
	if err != nil {
		return fmt.Errorf("could not execute delete letsencrypt statement: %w", err)
	}

	d.log.Debug("LetsEncrypt data deleted")

	return nil
}

func (d *db) RemoveSbcInfo(sbcFqdn string) error {
	stmt, err := d.db.Prepare(
		"SELECT sbc_info.id, k.id, re.id " +
			"FROM sbc_info " +
			"JOIN kamailio k on k.id = sbc_info.kamailio_id " +
			"JOIN rtp_engine re on re.id = sbc_info.rtp_engine_id " +
			"WHERE sbc_info.id == (" +
			"SELECT id from sbc_info WHERE fqdn = ?)")
	if err != nil {
		return fmt.Errorf("could not prepare select statement: %w", err)
	}

	row, err := stmt.Query(sbcFqdn)
	if err != nil {
		return fmt.Errorf("could not query select statement: %w", err)
	}

	sbcId := int64(0)
	kamId := int64(0)
	rtpId := int64(0)

	for row.Next() {
		if err = row.Scan(&sbcId, &kamId, &rtpId); err != nil {
			return fmt.Errorf("could not scan ids into vars: %w", err)
		}
	}

	d.deleteRowWithID("sbc_info", sbcId)
	d.deleteRowWithID("kamailio", kamId)
	d.deleteRowWithID("rtp_engine", rtpId)

	d.log.Debug("Deleted sbc information from database", "sbc_fqdn", sbcFqdn)

	return nil
}

func (d *db) GetContainerIDsFromSbcFqdn(sbcFqdn string) []string {
	stmt, err := d.db.Prepare(
		"SELECT k.container_id, r.container_id " +
			"FROM sbc_info " +
			"JOIN kamailio k ON k.id = sbc_info.kamailio_id " +
			"JOIN rtp_engine r ON sbc_info.rtp_engine_id = r.id " +
			"WHERE sbc_info.fqdn = ?")
	if err != err {
		d.log.Error("Could not prepare select statement", "err", err)

		return nil
	}

	ids := struct {
		kamailio  string
		rtpEngine string
	}{}

	if err = stmt.QueryRow(sbcFqdn).Scan(&ids.kamailio, &ids.rtpEngine); err != nil {
		d.log.Error("Could not query and scan prepared select statement", "err", err)

		return nil
	}

	_ = stmt.Close()

	return []string{ids.kamailio, ids.rtpEngine}
}

func (d *db) SaveContainerID(rowID int64, tableName, containerID string) error {
	row, err := d.insertOrUpdateContainerID(rowID, tableName, containerID)
	if err != nil {
		return fmt.Errorf("could not execute prepared insert statement: %w", err)
	}

	afRows, _ := row.RowsAffected()
	d.log.Debug("Container ID saved", "affected_rows", afRows)

	return nil
}

func (d *db) RevertLastInsert() {
	d.deleteRowWithID("sbc_info", d.insertID.sbcInstance)
	d.deleteRowWithID("kamailio", d.insertID.kamailio)
	d.deleteRowWithID("rtp_engine", d.insertID.rtpEngine)
}

func (d *db) SaveSBCInformation() (int64, error) {
	var err error

	// check if required flags are present
	if err := checkForRequiredFlags(); err != nil {
		d.log.Error("Required flags check failed", "err", err)
		return -1, err
	}

	// store kamailio config and save insert id
	if err = d.storeKamailioData(); err != nil {
		d.log.Error("Could not store kamailio data", "err", err)

		return -1, err
	}

	// store rtp engine config and save insert id
	if err = d.storeRtpEngineData(); err != nil {
		d.log.Error("Could not store rtp engine data", "err", err)

		return -1, err
	}

	// store sbc info using the kamailio and rtp engine ids
	if err = d.storeSbcInfo(); err != nil {
		d.log.Error("Could not store sbc configuration information")

		return -1, err
	}

	return d.insertID.sbcInstance, nil
}

func (d *db) storeSbcInfo() error {
	stmt, err := d.db.Prepare("INSERT INTO sbc_info(fqdn, kamailio_id, rtp_engine_id, created) " +
		"VALUES(?,?,?,datetime());")
	if err != nil {
		return fmt.Errorf("could not prepare insert statement err=%w", err)
	}

	res, err := stmt.Exec(
		viper.GetString(flagnames.SbcFqdn),
		d.insertID.kamailio,
		d.insertID.rtpEngine,
	)
	if err != nil {
		return fmt.Errorf("could not execute insert statement err=%w", err)
	}

	d.log.Info("Sbc configuration information successfully saved")

	d.insertID.sbcInstance, err = res.LastInsertId()
	if err != nil {
		return fmt.Errorf("could not get last insert id from insert into sbc_info err=%w", err)
	}

	d.log.Debug("SBC configuration inserted", "insert_id", d.insertID.sbcInstance)

	return nil
}

func (d *db) GetSBCIdFromFqdn(sbcFqdn string) int64 {
	var sbcID int64

	if err := d.db.QueryRowContext(
		context.Background(),
		"SELECT id FROM sbc_info WHERE fqdn = ?", sbcFqdn).Scan(&sbcID); err != nil {
		d.log.Error("Could not get sbc id", "fqdn", sbcFqdn, "err", err)
	}

	return sbcID
}

func (d *db) GetSBCParameters(sbcId int64) (types.Sbc, error) {
	stmt, err := d.db.Prepare(
		"SELECT " +
			"fqdn, sbc_name, sbc_tls_port, sbc_udp_port, " +
			"pbx_ip, pbx_port, rtp_engine_port, rtp_max, " +
			"rtp_min, media_public_ip, ng_listen, new_config, enable_sipdump " +
			"FROM sbc_info " +
			"JOIN kamailio k ON k.id = sbc_info.kamailio_id " +
			"JOIN rtp_engine re ON re.id = sbc_info.rtp_engine_id " +
			"WHERE sbc_info.id == ?")
	if err != nil {
		return types.Sbc{}, fmt.Errorf("could not prepare select statement err=%w", err)
	}

	res, err := stmt.Query(sbcId)
	if err != nil {
		return types.Sbc{}, fmt.Errorf("could not run query for select statement err=%w", err)
	}

	sbcResult := types.Sbc{}

	for res.Next() {
		if err := res.Scan(
			&sbcResult.Fqdn,
			&sbcResult.SbcName,
			&sbcResult.SbcTLSPort,
			&sbcResult.SbcUDPPort,
			&sbcResult.PbxIP,
			&sbcResult.PbxPort,
			&sbcResult.RtpEnginePort,
			&sbcResult.RtpMaxPort,
			&sbcResult.RtpMinPort,
			&sbcResult.MediaPublicIP,
			&sbcResult.NgListen,
			&sbcResult.NewConfig,
			&sbcResult.EnableSipDump,
		); err != nil {
			return types.Sbc{}, fmt.Errorf("could not scan data into struct err=%w", err)
		}

	}

	d.log.Debug("Data fetched from database", "data", sbcResult)

	return sbcResult, nil
}

func (d *db) storeRtpEngineData() error {
	// select the last record in the table
	rows, err := d.db.Query("SELECT * FROM rtp_engine ORDER BY id DESC LIMIT 1;")
	if err != nil {
		return err
	}

	var (
		rtpMaxPort, rtpMinPort, rtpSignalPort string
		rtpPubIP                              = viper.GetString(flagnames.RtpPublicIp)
	)

	// check if the table is empty
	if !rows.Next() {
		d.log.Debug("Rtp engine table is empty, using default first data")

		rtpMaxPort = viper.GetString(flagnames.RtpMaxPort)
		rtpMinPort = viper.GetString(flagnames.RtpMinPort)
		rtpSignalPort = viper.GetString(flagnames.RtpSignalPort)
	} else {
		rtpSignalPort = d.getSingleRecordAndIncreaseByValue(1, "ng_listen", "rtp_engine")
		rtpMinPort = d.getSingleRecordAndIncreaseByValue(500, "rtp_min", "rtp_engine")
		rtpMaxPort = d.getSingleRecordAndIncreaseByValue(500, "rtp_max", "rtp_engine")
	}

	_ = rows.Close()

	stmt, err := d.db.Prepare("INSERT INTO rtp_engine(rtp_max, rtp_min, media_public_ip, ng_listen) " +
		"VALUES (?,?,?,?);")
	if err != nil {
		return fmt.Errorf("could not prepare insert statement err=%w", err)
	}

	res, err := stmt.Exec(
		rtpMaxPort,
		rtpMinPort,
		rtpPubIP,
		rtpSignalPort,
	)
	if err != nil {
		return fmt.Errorf("could not execute prepared statement err=%w", err)
	}

	d.insertID.rtpEngine, err = res.LastInsertId()
	if err != nil {
		return fmt.Errorf("could not get last inserted id err=%w", err)
	}

	d.log.Debug("RTP engine configuration inserted", "insert_id", d.insertID.kamailio)

	return nil
}

func (d *db) storeKamailioData() error {
	// get Kamailio values from flags
	var (
		newConfig                       int
		enableSipDump                   int
		sbcPort, udpSipPort, rtpEngPort string
		// user must always set these values and/or they don't have to be unique
		pbxIP   = viper.GetString(flagnames.KamailioPbxIp)
		pbxPort = viper.GetString(flagnames.KamailioPbxPort)
		sbcFqdn = viper.GetString(flagnames.SbcFqdn)
	)

	// translate bool to int
	if viper.GetBool(flagnames.KamailioNewConfig) {
		newConfig = 1
	}

	if viper.GetBool(flagnames.KamailioSipDump) {
		enableSipDump = 1
	}

	// select the last record in the table
	qRes, err := d.db.Query("SELECT * FROM kamailio ORDER BY sbc_name DESC LIMIT 1;")
	if err != nil {
		return err
	}

	// check if the table is empty
	if !qRes.Next() {
		d.log.Debug("Kamailio table is empty, using default first data")

		// if there are no records in the table, default values will be used
		rtpEngPort = viper.GetString(flagnames.KamailioRtpEngPort)
		sbcPort = viper.GetString(flagnames.KamailioSbcPort)
		udpSipPort = viper.GetString(flagnames.KamailioUdpSipPort)
	} else {
		d.log.Debug("Existing Kamailio data found, calculating next values")

		// if there are some records, next values will be calculated
		rtpEngPort = d.getSingleRecordAndIncreaseByValue(1, "rtp_engine_port", "kamailio")
		udpSipPort = d.getSingleRecordAndIncreaseByValue(1, "sbc_udp_port", "kamailio")
		sbcPort = d.getSingleRecordAndIncreaseByValue(1, "sbc_tls_port", "kamailio")
	}

	_ = qRes.Close()

	// prepare statement
	stmt, err := d.db.Prepare(
		"INSERT INTO kamailio(new_config, enable_sipdump, pbx_ip, pbx_port, rtp_engine_port, sbc_name, sbc_tls_port, sbc_udp_port) " +
			"VALUES (?,?,?,?,?,?,?,?);")
	if err != nil {
		return fmt.Errorf("could not prepare insert statement err=%w", err)
	}

	// pass flag values and execute
	res, err := stmt.Exec(
		newConfig,
		enableSipDump,
		pbxIP,
		pbxPort,
		rtpEngPort,
		sbcFqdn,
		sbcPort,
		udpSipPort,
	)
	if err != nil {
		return fmt.Errorf("could not execute insert statement err=%w", err)
	}

	// save insert id
	d.insertID.kamailio, err = res.LastInsertId()
	if err != nil {
		return fmt.Errorf("could not get last insert id err=%w", err)
	}

	d.log.Debug("Kamailio configuration inserted", "insert_id", d.insertID.kamailio)

	return nil
}

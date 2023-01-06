package db

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/ZeljkoBenovic/tsbc/cmd/flagnames"
	"github.com/hashicorp/go-hclog"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/viper"
)

type IDB interface {
	CreateFreshDB() error
	SaveSBCInformation() (int64, error)
	GetSBCParameters(sbcId int64) (Sbc, error)
	Close() error
	RevertLastInsert()
	SaveContainerID(rowID int64, tableName, id string) error
	GetKamailioInsertID() int64
	GetRTPEngineInsertID() int64
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

func NewDB(logger hclog.Logger) (IDB, error) {
	var err error
	dbInstance := &db{
		log: logger.Named("db"),
	}

	dbInstance.log.Debug("Creating new SQLite instance")

	dbInstance.db, err = sql.Open("sqlite3", "sbc.db")
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

func (d *db) GetKamailioInsertID() int64 {
	return d.insertID.kamailio
}

func (d *db) GetRTPEngineInsertID() int64 {
	return d.insertID.rtpEngine
}

func (d *db) SaveContainerID(rowID int64, tableName, containerID string) error {
	stmt, err := d.db.Prepare(fmt.Sprintf("UPDATE %s SET container_id = ? WHERE id = ?", tableName))
	if err != nil {
		return fmt.Errorf("could not prepare insert statement: %w", err)
	}

	row, err := stmt.Exec(containerID, rowID)
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

func (d *db) CreateFreshDB() error {
	var err error

	// check if there is data in the table already
	tableExists, err := d.checkIfTableExists("sbc_info")
	if err != nil {
		return fmt.Errorf("error while checking if table already exists err=%w", err)
	}

	if tableExists {
		d.log.Debug("Table already exists, skipping new schema generation", "table", "sbc_info")

		// return nil as this is not an error, and we want the rest of the program to continue
		return nil
	}

	// TODO: set appropriate types
	schema := `create table kamailio
(
    id              INTEGER
        constraint kamailio_pk
            primary key autoincrement,
    new_config      INTEGER default 0,
    enable_sipdump  INTEGER default 0,
    sbc_name        TEXT not null,
    sbc_tls_port    TEXT not null,
    sbc_udp_port    TEXT not null,
    pbx_ip          TEXT not null,
    pbx_port        TEXT not null,
    rtp_engine_port TEXT not null,
	container_id    TEXT DEFAULT null
);

create unique index kamailio_id_uindex
    on kamailio (id);

create unique index kamailio_rtp_engine_port_uindex
    on kamailio (rtp_engine_port);

create unique index kamailio_sbc_name_uindex
    on kamailio (sbc_name);

create unique index kamailio_sbc_tls_port_uindex
    on kamailio (sbc_tls_port);

create unique index kamailio_sbc_udp_port_uindex
    on kamailio (sbc_udp_port);

create table rtp_engine
(
    id              INTEGER
        constraint rtp_engine_pk
            primary key autoincrement,
    rtp_max         TEXT not null,
    rtp_min         TEXT not null,
    media_public_ip TEXT not null,
    ng_listen       TEXT not null,
	container_id    TEXT default null
);

create table sbc_info
(
    id            INTEGER
        primary key autoincrement,
    fqdn          TEXT not null,
    created       DATE not null,
    kamailio_id   INTEGER not null
        references kamailio 
            on delete cascade,
    rtp_engine_id INTEGER not null
        references rtp_engine 
            on delete cascade
);`

	d.log.Debug("Creating new db schema")

	_, err = d.db.Exec(schema)
	if err != nil {
		d.log.Error("Could run create schema query", "err", err)

		return err
	}

	d.log.Debug("New db schema created")

	return nil
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

func (d *db) GetSBCParameters(sbcId int64) (Sbc, error) {
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
		return Sbc{}, fmt.Errorf("could not prepare select statement err=%w", err)
	}

	res, err := stmt.Query(sbcId)
	if err != nil {
		return Sbc{}, fmt.Errorf("could not run query for select statement err=%w", err)
	}

	sbcResult := Sbc{}

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
			return Sbc{}, fmt.Errorf("could not scan data into struct err=%w", err)
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

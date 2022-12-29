package db

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/hashicorp/go-hclog"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/viper"
)

type IDB interface {
	CreateFreshDB() error
	SaveSBCInformation() error
}

var (
	ErrPbxIpNotDefined             = errors.New("pbx ip address not defined")
	ErrSbcFqdnNotDefined           = errors.New("sbc fqdn not defined")
	ErrRtpEnginePublicIPNotDefined = errors.New("rtp engine public ip address not defined")
)

type db struct {
	dataSource string
	db         *sql.DB
	log        hclog.Logger

	insertID
}

type insertID struct {
	kamailio, rtpEngine int64
}

func NewDB(logger hclog.Logger) IDB {
	return &db{
		dataSource: "sbc.db",
		log:        logger.Named("db"),
	}
}

func (d *db) CreateFreshDB() error {
	var err error

	// TODO: set appropriate types
	schema := `create table kamailio
(
    new_config      INTEGER default 0,
    enable_sipdump  INTEGER default 0,
    id              INTEGER
        constraint kamailio_pk
            primary key autoincrement,
    sbc_name        TEXT not null,
    sbc_tls_port    TEXT not null,
    sbc_udp_port    TEXT not null,
    pbx_ip          TEXT not null,
    pbx_port        TEXT not null,
    rtp_engine_port TEXT not null
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
    ng_listen       TEXT not null
);

create table sbc_info
(
    id            INTEGER
        primary key autoincrement,
    fqdn          TEXT not null,
    created       DATE not null,
    kamailio_id   INTEGER not null
        references kamailio,
    rtp_engine_id INTEGER not null
        references rtp_engine
);`

	d.log.Debug("Creating new SQLite instance")

	d.db, err = sql.Open("sqlite3", d.dataSource)
	if err != nil {
		d.log.Error("Could not open db connection", "data_source", d.dataSource, "err", err)

		return err
	}

	d.log.Debug("SQLite instance created")

	d.log.Debug("Creating new db schema")

	_, err = d.db.Exec(schema)
	if err != nil {
		d.log.Error("Could run create schema query", "err", err)

		return err
	}

	d.log.Debug("New db schema created")

	_ = d.db.Close()

	return nil
}

func (d *db) SaveSBCInformation() error {
	var err error

	// check if required flags are present
	if err := checkForRequiredFlags(); err != nil {
		return err
	}

	// open db connection
	d.db, err = sql.Open("sqlite3", d.dataSource)
	if err != nil {
		d.log.Error("Could not open db connection", "data_source", d.dataSource, "err", err)

		return err
	}

	// store kamailio config and save insert id
	if err = d.storeKamailioData(); err != nil {
		d.log.Error("Could not store kamailio data", "err", err)

		return err
	}

	// store rtp engine config and save insert id
	if err = d.storeRtpEngineData(); err != nil {
		d.log.Error("Could not store rtp engine data", "err", err)

		return err
	}

	// store sbc info using the kamailio and rtp engine ids
	if err = d.storeSbcInfo(); err != nil {
		d.log.Error("Could not store sbc configuration information")

		return err
	}

	return nil
}

func (d *db) storeSbcInfo() error {
	stmt, err := d.db.Prepare("INSERT INTO sbc_info(fqdn, kamailio_id, rtp_engine_id, created) " +
		"VALUES(?,?,?,datetime());")
	if err != nil {
		return fmt.Errorf("could not prepare insert statement err=%w", err)
	}

	_, err = stmt.Exec(
		viper.GetString("sbc-fqdn"),
		d.insertID.kamailio,
		d.insertID.rtpEngine,
	)
	if err != nil {
		return fmt.Errorf("could not execute insert statement err=%w", err)
	}

	d.log.Info("Sbc configuration information successfully saved")

	return nil
}

func (d *db) storeRtpEngineData() error {
	// select the last record in the table
	rows, err := d.db.Query("SELECT * FROM rtp_engine ORDER BY id DESC LIMIT 1;")
	if err != nil {
		return err
	}

	var (
		rtpMaxPort, rtpMinPort, rtpSignalPort string
		rtpPubIP                              = viper.GetString("rtp-public-ip")
	)

	// check if the table is empty
	if !rows.Next() {
		d.log.Debug("Rtp engine table is empty, using default first data")

		rtpMaxPort = viper.GetString("rtp-max-port")
		rtpMinPort = viper.GetString("rtp-min-port")
		rtpSignalPort = viper.GetString("rtp-signal-port")
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
		pbxIP   = viper.GetString("kamailio-pbx-ip")
		pbxPort = viper.GetString("kamailio-pbx-port")
		sbcFqdn = viper.GetString("sbc-fqdn")
	)

	// translate bool to int
	if viper.GetBool("kamailio-new-config") {
		newConfig = 1
	}

	if viper.GetBool("kamailio-sip-dump") {
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
		rtpEngPort = viper.GetString("kamailio-rtpeng-port")
		sbcPort = viper.GetString("kamailio-sbc-port")
		udpSipPort = viper.GetString("kamailio-udp-sip-port")
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

func checkForRequiredFlags() error {
	// pbx ip can not be undefined
	if viper.GetString("kamailio-pbx-ip") == "" {
		return ErrPbxIpNotDefined
	}

	// sbc name can not be undefined
	if viper.GetString("sbc-fqdn") == "" {
		return ErrSbcFqdnNotDefined
	}

	// TODO: add checks for valid IP address format
	// public ip address must be defined
	if viper.GetString("rtp-public-ip") == "" {
		return ErrRtpEnginePublicIPNotDefined
	}

	return nil
}

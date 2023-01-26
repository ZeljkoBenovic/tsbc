package db

import "fmt"

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

create table letsencrypt
(
    id INTEGER primary key autoincrement,
    container_id    TEXT not null
);
/*insert dummy data into letsencrypt as it will get overridden*/
INSERT INTO letsencrypt (container_id) VALUES ('');
                                                   
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

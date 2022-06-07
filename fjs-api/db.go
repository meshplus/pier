package fjs_api

import (
	"github.com/jmoiron/sqlx"
)

func (g *FjsServer) createDB(db *sqlx.DB) error {
	//创建表
	sql_table := `
    CREATE TABLE IF NOT EXISTS ibtp(
        uid INTEGER PRIMARY KEY AUTOINCREMENT,
        ibtpid VARCHAR(64) NULL unique ,
        created TIMESTAMP DEFAULT (datetime('now','localtime'))
    );

  	CREATE TABLE IF NOT EXISTS ibtp_crsChnTxProc(
        uid INTEGER PRIMARY KEY AUTOINCREMENT,
        ibtpid VARCHAR(64) NULL unique ,
        created TIMESTAMP DEFAULT (datetime('now','localtime'))
    );

  	CREATE TABLE IF NOT EXISTS ibtp_crsChnTxFail(
        uid INTEGER PRIMARY KEY AUTOINCREMENT,
        ibtpid VARCHAR(64) NULL unique ,
        created TIMESTAMP DEFAULT (datetime('now','localtime'))
    );  

	

	CREATE TABLE IF NOT EXISTS ibtp_count(
        uid INTEGER PRIMARY KEY AUTOINCREMENT,
        amount INTEGER default 0,
        created TIMESTAMP DEFAULT (datetime('now','localtime'))
    );

  	CREATE TABLE IF NOT EXISTS ibtp_crsChnTxProc_count(
        uid INTEGER PRIMARY KEY AUTOINCREMENT,
        amount INTEGER default 0,
        created TIMESTAMP DEFAULT (datetime('now','localtime'))
    );

  	CREATE TABLE IF NOT EXISTS ibtp_crsChnTxFail_count(
        uid INTEGER PRIMARY KEY AUTOINCREMENT,
        amount INTEGER default 0,
        created TIMESTAMP DEFAULT (datetime('now','localtime'))
    );
    `
	_, err := db.Exec(sql_table)
	return err
}

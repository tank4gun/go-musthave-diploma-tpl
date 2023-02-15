package varprs

import (
	"flag"
	"log"
	"os"
)

var ServerAddr string
var DBURI string
var AccrualSysAddr string

func Init() {
	flag.StringVar(&ServerAddr, "a", "localhost:8087", "GopherMart server address")
	flag.StringVar(&DBURI, "d", "postgresql://postgres2:GophermartPass@localhost:6432/gophermart?sslmode=disable", "GopherMart database address")
	flag.StringVar(&AccrualSysAddr, "r", "http://localhost:8080", "Accrual system address")
	flag.Parse()

	ServerAddrEnv := os.Getenv("RUN_ADDRESS")
	if ServerAddrEnv != "" {
		ServerAddr = ServerAddrEnv
	}

	DBURIEnv := os.Getenv("DATABASE_URI")
	if DBURIEnv != "" {
		DBURI = DBURIEnv
	}

	AccrualSysAddrEnv := os.Getenv("ACCRUAL_SYSTEM_ADDRESS")
	if AccrualSysAddrEnv != "" {
		AccrualSysAddr = AccrualSysAddrEnv
	}

	log.Printf("Got ServerAddr %s, DBURI %s, AccrualSysAddr %s to run GopherMart", ServerAddr, DBURI, AccrualSysAddr)
}

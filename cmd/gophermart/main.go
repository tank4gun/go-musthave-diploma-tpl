package main

import (
	"github.com/tank4gun/go-musthave-diploma-tpl/internal/db"
	"github.com/tank4gun/go-musthave-diploma-tpl/internal/server"
	"github.com/tank4gun/go-musthave-diploma-tpl/internal/storage"
	"github.com/tank4gun/go-musthave-diploma-tpl/internal/varprs"
)

func main() {
	varprs.Init()
	db.RunMigrations(varprs.DBURI)
	storageForHandler := storage.GetStorage(varprs.DBURI)
	serverToRun := server.CreateServer(storageForHandler)
	serverToRun.ListenAndServe()
}

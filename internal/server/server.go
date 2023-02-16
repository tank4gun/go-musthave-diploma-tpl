package server

import (
	"github.com/go-chi/chi/v5"
	"github.com/tank4gun/go-musthave-diploma-tpl/internal/handlers"
	"github.com/tank4gun/go-musthave-diploma-tpl/internal/storage"
	"github.com/tank4gun/go-musthave-diploma-tpl/internal/varprs"
	"net/http"
)

func CreateServer(storageForHandler storage.Storage) *http.Server {
	router := chi.NewRouter()

	handlerWithStorage := handlers.GetHandlerWithStorage(storageForHandler)
	router.Use(handlers.CheckAuth)
	go handlerWithStorage.GetStatusesDaemon()
	router.Post("/api/user/register", handlerWithStorage.Register)
	router.Post("/api/user/login", handlerWithStorage.Login)
	router.Post("/api/user/orders", handlerWithStorage.AddOrder)
	router.Get("/api/user/orders", handlerWithStorage.GetOrders)
	router.Get("/api/user/balance", handlerWithStorage.GetBalance)
	router.Post("/api/user/balance/withdraw", handlerWithStorage.AddWithdrawal)
	router.Get("/api/user/withdrawals", handlerWithStorage.GetWithdrawals)

	server := &http.Server{
		Addr:    varprs.ServerAddr,
		Handler: router,
	}
	return server
}

package storage

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"log"
	"net/http"
	"time"
)

type UserAuthData struct {
	Login    string `json:"login"`
	Password string `json:"password"`
	UserID   string `json:"userID,omitempty"`
}

type Order struct {
	Number     string    `json:"number"`
	Status     string    `json:"status"`
	Accrual    float64   `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
}

type OrderFromDB struct {
	Number     string
	Status     string
	Accrual    sql.NullFloat64
	UploadedAt time.Time
}

type OrderFromBlackBox struct {
	Order   string  `json:"order"`
	Status  string  `json:"status"`
	Accrual float64 `json:"accrual,omitempty"`
}

type UserBalance struct {
	Orders    float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

type Withdrawal struct {
	Order       string    `json:"order"`
	Sum         float64   `json:"sum"`
	ProcessedAt time.Time `json:"processed_at,omitempty"`
}

func GetStorage(dbDSN string) Storage {
	db, err := sql.Open("pgx", dbDSN)
	if err != nil {
		log.Printf("Got error while starting db %s", err.Error())
		return nil
	}
	return &DBStorage{db}
}

type Storage interface {
	Register(registerData UserAuthData) (string, int)
	GetUserByLogin(authData UserAuthData) (UserAuthData, int)
	GetOrdersByUser(userID string) ([]Order, int)
	AddOrderForUser(externalOrderID string, userID string) int
	GetUserBalance(userID string) (UserBalance, int)
	AddWithdrawalForUser(userID string, withdrawal Withdrawal) int
	GetWithdrawalsForUser(userID string) ([]Withdrawal, int)
	GetOrdersInProgress() ([]Order, int)
	UpdateOrder(order OrderFromBlackBox) int
}

type DBStorage struct {
	db *sql.DB
}

func (strg *DBStorage) Register(registerData UserAuthData) (string, int) {
	row := strg.db.QueryRow("SELECT id FROM \"user\" WHERE \"login\" = $1", registerData.Login)
	var userID sql.NullString
	err := row.Scan(&userID)
	if err != nil && userID.Valid {
		log.Printf("Got error %s", err.Error())
		return "", http.StatusInternalServerError
	}
	if userID.Valid {
		log.Printf("Got existing user with login %s", registerData.Login)
		return "", http.StatusFailedDependency
	}
	h := sha256.New()
	h.Write([]byte(registerData.Password))
	passwordHash := hex.EncodeToString(h.Sum(nil))
	log.Printf("Got password hash %s", passwordHash)
	row = strg.db.QueryRow("INSERT INTO \"user\" (\"login\", password_hash) VALUES ($1, $2) RETURNING id", registerData.Login, passwordHash)
	if err := row.Scan(&userID); err != nil {
		log.Printf("Error %s", err.Error())
		return "", http.StatusInternalServerError
	} else {
		log.Printf("Got userID %v", userID)
		if userID.Valid {
			userIDValue := userID.String
			log.Printf("Got new userID %s", userIDValue)
			return userIDValue, http.StatusOK
		}
	}
	return "", http.StatusInternalServerError
}

func (strg *DBStorage) GetUserByLogin(authData UserAuthData) (UserAuthData, int) {
	row := strg.db.QueryRow("SELECT id, login, password_hash FROM \"user\" WHERE login = $1", authData.Login)
	var userData UserAuthData
	err := row.Scan(&userData.UserID, &userData.Login, &userData.Password)
	if err != nil {
		log.Printf("Could not get user data for login %s", authData.Login)
		return userData, http.StatusUnauthorized
	}
	return userData, http.StatusOK
}

func (strg *DBStorage) AddOrderForUser(externalOrderID string, userID string) int {
	row := strg.db.QueryRow("SELECT user_id FROM \"order\" WHERE external_id = $1", externalOrderID)
	var orderUserID sql.NullString
	err := row.Scan(&orderUserID)
	if err != nil && orderUserID.Valid {
		log.Printf("Got error while querying %s", err.Error())
		return http.StatusInternalServerError
	}
	if orderUserID.Valid {
		if orderUserID.String == userID {
			log.Printf("Got same userID %s for orderID %s", userID, externalOrderID)
			return http.StatusOK
		} else {
			log.Printf("Got another userID %s (instead of %s) for orderID %s", orderUserID.String, userID, externalOrderID)
			return http.StatusConflict
		}
	}
	log.Printf("Order with id %v not found in DB, should add it", externalOrderID)
	row = strg.db.QueryRow(
		"INSERT INTO \"order\" (user_id, status, external_id) VALUES ($1, $2, $3) RETURNING id",
		userID, "NEW", externalOrderID,
	)
	var orderID string
	err = row.Scan(&orderID)
	if err != nil {
		log.Printf("Smth went wrong while adding new order: %s", err.Error())
		return http.StatusInternalServerError
	}
	log.Printf("New order with id %s added", orderID)
	return http.StatusAccepted
}

func (strg *DBStorage) GetOrdersByUser(userID string) ([]Order, int) {
	rows, err := strg.db.Query("SELECT external_id, status, amount, registered_at FROM \"order\" WHERE user_id = $1", userID)
	if err != nil {
		log.Printf("Got error: %s", err.Error())
		return nil, http.StatusInternalServerError
	}
	defer rows.Close()
	orders := make([]Order, 0)
	for rows.Next() {
		var orderFromDBVal OrderFromDB
		err := rows.Scan(&orderFromDBVal.Number, &orderFromDBVal.Status, &orderFromDBVal.Accrual, &orderFromDBVal.UploadedAt)
		if err != nil {
			log.Printf("Got error: %s", err.Error())
			return nil, http.StatusInternalServerError
		}
		order := Order{Number: orderFromDBVal.Number, Status: orderFromDBVal.Status, UploadedAt: orderFromDBVal.UploadedAt}
		if orderFromDBVal.Accrual.Valid {
			order.Accrual = orderFromDBVal.Accrual.Float64
		}
		orders = append(orders, order)
	}
	if err := rows.Err(); err != nil {
		log.Printf("Got error: %s", err.Error())
		return nil, http.StatusInternalServerError
	}
	return orders, http.StatusOK
}

func (strg *DBStorage) GetUserBalance(userID string) (UserBalance, int) {
	log.Printf("Got userID %s", userID)
	sumOrdersRow := strg.db.QueryRow("SELECT sum(amount) FROM \"order\" WHERE user_id = $1", userID)
	sumWithdrawalsRow := strg.db.QueryRow("SELECT sum(amount) FROM withdrawal WHERE user_id = $1", userID)
	var sumOrders sql.NullFloat64
	var sumWithdrawals sql.NullFloat64
	err := sumOrdersRow.Scan(&sumOrders)
	if err != nil && sumOrders.Valid {
		log.Printf("Could not get sumOrders: %s", err.Error())
		return UserBalance{0, 0}, http.StatusInternalServerError
	}
	var resultBalance UserBalance
	if !sumOrders.Valid {
		log.Printf("Got empty resultBalance")
		resultBalance.Orders = 0
	} else {
		resultBalance.Orders = sumOrders.Float64
	}
	err = sumWithdrawalsRow.Scan(&sumWithdrawals)

	if err != nil && sumWithdrawals.Valid {
		log.Printf("Could not get sumWithdrawals: %s", err.Error())
		return UserBalance{0, 0}, http.StatusInternalServerError
	}
	if !sumWithdrawals.Valid {
		log.Printf("Got empty resultBalance")
		resultBalance.Withdrawn = 0
	} else {
		resultBalance.Withdrawn = sumWithdrawals.Float64
	}
	resultBalance.Orders -= resultBalance.Withdrawn
	log.Printf("Got balance %v", resultBalance)
	return resultBalance, http.StatusOK
}

func (strg *DBStorage) AddWithdrawalForUser(userID string, withdrawal Withdrawal) int {
	userBalance, errCode := strg.GetUserBalance(userID)
	if errCode != http.StatusOK {
		log.Printf("Got error while getting status %v", errCode)
		return errCode
	}
	if userBalance.Orders < withdrawal.Sum {
		log.Printf("Got less bonus points %v than expected %v", userBalance.Orders, withdrawal.Sum)
		return http.StatusPaymentRequired
	}
	var withdrawalID string
	row := strg.db.QueryRow(
		"INSERT INTO withdrawal (user_id, amount, external_id) VALUES ($1, $2, $3) RETURNING id",
		userID, withdrawal.Sum, withdrawal.Order,
	)
	err := row.Scan(&withdrawalID)
	if err != nil {
		log.Printf("Got error %s", err.Error())
		return http.StatusInternalServerError
	}
	log.Printf("Got new withdrawal %s", withdrawalID)
	return http.StatusOK
}

func (strg *DBStorage) GetWithdrawalsForUser(userID string) ([]Withdrawal, int) {
	rows, err := strg.db.Query("SELECT external_id, amount, registered_at FROM withdrawal WHERE user_id = $1", userID)
	if err != nil {
		log.Printf("Got error %s", err.Error())
		return make([]Withdrawal, 0), http.StatusInternalServerError
	}
	defer rows.Close()
	withdrawals := make([]Withdrawal, 0)
	for rows.Next() {
		var withdrawal Withdrawal
		err = rows.Scan(&withdrawal.Order, &withdrawal.Sum, &withdrawal.ProcessedAt)
		if err != nil {
			log.Printf("Got error %s", err.Error())
			return make([]Withdrawal, 0), http.StatusInternalServerError
		}
		withdrawals = append(withdrawals, withdrawal)
	}
	if err := rows.Err(); err != nil {
		log.Printf("Got error: %s", err.Error())
		return nil, http.StatusInternalServerError
	}
	return withdrawals, http.StatusOK
}

func (strg *DBStorage) GetOrdersInProgress() ([]Order, int) {
	rows, err := strg.db.Query("SELECT external_id, status, amount from \"order\" where status not in ('INVALID', 'PROCESSED')")

	if err != nil {
		log.Printf("Got error %s", err.Error())
		return make([]Order, 0), http.StatusInternalServerError
	}
	defer rows.Close()
	orders := make([]Order, 0)
	for rows.Next() {
		var orderFromDBVal OrderFromDB
		err = rows.Scan(&orderFromDBVal.Number, &orderFromDBVal.Status, &orderFromDBVal.Accrual)
		if err != nil {
			log.Printf("Got error %s", err.Error())
			return make([]Order, 0), http.StatusInternalServerError
		}
		order := Order{Number: orderFromDBVal.Number, Status: orderFromDBVal.Status}
		if orderFromDBVal.Accrual.Valid {
			order.Accrual = orderFromDBVal.Accrual.Float64
		}
		orders = append(orders, order)
	}
	if err := rows.Err(); err != nil {
		log.Printf("Got error: %s", err.Error())
		return nil, http.StatusInternalServerError
	}
	log.Printf("Got orders %v", orders)
	return orders, http.StatusOK
}

func (strg *DBStorage) UpdateOrder(order OrderFromBlackBox) int {
	tx, err := strg.db.Begin()
	if err != nil {
		log.Printf("Got error %s", err.Error())
		return http.StatusInternalServerError
	}
	URLstmt, err := tx.Prepare("UPDATE \"order\" SET status = $1, amount = $2 where external_id = $3")
	if err != nil {
		log.Printf("Got error %s", err.Error())
		return http.StatusInternalServerError
	}
	defer URLstmt.Close()
	if _, err := URLstmt.Exec(order.Status, order.Accrual, order.Order); err != nil {
		if err = tx.Rollback(); err != nil {
			log.Fatalf("Insert to url, need rollback, %v", err)
			return http.StatusInternalServerError
		}
		return http.StatusInternalServerError
	}
	if err := tx.Commit(); err != nil {
		log.Fatalf("Unable to commit: %v", err)
		return http.StatusInternalServerError
	}
	return http.StatusOK
}

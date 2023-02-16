package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/tank4gun/go-musthave-diploma-tpl/internal/storage"
	"github.com/tank4gun/go-musthave-diploma-tpl/internal/varprs"
	"io"
	"net/http"
	"strconv"
	"time"
)

var UserCookie = "UserCookie"
var UserID = "UserID"
var CookieKey = []byte("SecretKeyToUserID")

type HandlerWithStorage struct {
	storage storage.Storage
	client  http.Client
}

func GetHandlerWithStorage(storage storage.Storage) HandlerWithStorage {
	return HandlerWithStorage{storage: storage, client: http.Client{}}
}

func ValidateOrder(order string) (uint, int) {
	orderNum, err := strconv.Atoi(order)
	if err != nil {
		fmt.Printf("Got err while casting order to int %s", err.Error())
		return 0, http.StatusBadRequest
	}
	if orderNum < 0 {
		fmt.Println("Got orderNum < 0")
		return 0, http.StatusBadRequest
	}
	sum := 0
	if len(order)%2 == 0 {
		for i, num := range []rune(order) {
			if i%2 == 0 {
				if int(num-'0')*2 > 9 {
					sum += int(num-'0')*2 - 9
				} else {
					sum += int(num-'0') * 2
				}
			} else {
				sum += int(num - '0')
			}
		}
	} else {
		for i, num := range []rune(order) {
			if i%2 == 1 {
				if int(num-'0')*2 > 9 {
					sum += int(num-'0')*2 - 9
				} else {
					sum += int(num-'0') * 2
				}
			} else {
				sum += int(num - '0')
			}
		}
	}
	if sum%10 == 0 {
		return uint(orderNum), http.StatusOK
	} else {
		return 0, http.StatusUnprocessableEntity
	}
}

func CheckAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/user/register" || r.URL.Path == "/api/user/login" {
			fmt.Printf("Got %s url, skip check", r.URL.Path)
			next.ServeHTTP(w, r)
			return
		}
		cookie, err := (*r).Cookie(UserCookie)
		if cookie != nil && err != nil {
			fmt.Println(err.Error())
			http.Error(w, "Could not auth user", http.StatusUnauthorized)
			return
		}
		if cookie == nil {
			fmt.Println("Got null value in Cookie for UserID")
			http.Error(w, "Could not auth user", http.StatusUnauthorized)
			return
		}
		//data := cookie.Value
		data, err := hex.DecodeString(cookie.Value)
		fmt.Printf("Got Cookie: %s", cookie.Value)
		if err != nil {
			fmt.Println(err.Error())
			http.Error(w, "Could not auth user", http.StatusUnauthorized)
			return
		}
		h := hmac.New(sha256.New, CookieKey)
		h.Write(data[:36])
		sign := h.Sum(nil)
		if hmac.Equal(sign, data[36:]) {
			ctx := context.WithValue(r.Context(), UserID, string(data[:36]))
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		fmt.Println("Got not equal sign for UserID")
		http.Error(w, "Could not auth user", http.StatusUnauthorized)
		return
	})
}

func (strg *HandlerWithStorage) GetStatusesDaemon() {
	for {
		orders, _ := strg.storage.GetOrdersInProgress()
		//if errCode != http.StatusOK {
		//
		//}
		var ordersToSave = make([]storage.Order, 0)
		for _, order := range orders {
			response, err := strg.client.Get(varprs.AccrualSysAddr + "/api/orders/" + order.Number)
			if err != nil {
				fmt.Printf("Got error %s", err.Error())
				continue
			}
			if response.StatusCode == http.StatusOK {
				var newOrder storage.Order
				data, err := io.ReadAll(response.Body)
				if err != nil {
					fmt.Printf("Got error %s", err.Error())
					continue
				}
				err = json.Unmarshal(data, &newOrder)
				if err != nil {
					fmt.Printf("Got error %s", err.Error())
					continue
				}
				ordersToSave = append(ordersToSave, newOrder)
			} else {
				fmt.Printf("Got bad status code %v", response.StatusCode)
			}
		}
		strg.storage.UpdateOrders(ordersToSave)
		time.Sleep(1 * time.Second)
	}
}

func (strg *HandlerWithStorage) Register(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	fmt.Println("ADDD")
	jsonBody, err := io.ReadAll(r.Body)
	if err != nil {
		fmt.Printf("Got err while reading body: %s", err.Error())
		http.Error(w, "Got err while reading body", http.StatusBadRequest)
		return
	}
	var authData storage.UserAuthData
	fmt.Printf("DATA: %v", r.Body)
	err = json.Unmarshal(jsonBody, &authData)
	if err != nil {
		fmt.Printf("Could not unmarshal body: %s", err.Error())
		http.Error(w, "Could not unmarshal body", http.StatusBadRequest)
		return
	}
	userID, errCode := strg.storage.Register(authData)
	if errCode != http.StatusOK {
		fmt.Println("Could not register user")
		http.Error(w, "Could not register user", errCode)
		return
	}
	h := hmac.New(sha256.New, CookieKey)
	h.Write([]byte(userID))
	sign := h.Sum(nil)
	newCookie := http.Cookie{Name: UserCookie, Value: hex.EncodeToString(append([]byte(userID)[:], sign[:]...))}
	fmt.Printf("Sign %v, wholeCookie %v", sign, []byte(userID))
	http.SetCookie(w, &newCookie)
	w.WriteHeader(http.StatusOK)
	w.Write(make([]byte, 0))
}

func (strg *HandlerWithStorage) Login(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	jsonData, err := io.ReadAll(r.Body)
	if err != nil {
		fmt.Printf("Got err while reading body: %s", err.Error())
		http.Error(w, "Got err while reading body", http.StatusBadRequest)
		return
	}
	var authData storage.UserAuthData
	err = json.Unmarshal(jsonData, &authData)
	if err != nil {
		fmt.Printf("Could not unmarshal body: %s", err.Error())
		http.Error(w, "Could not unmarshal body", http.StatusBadRequest)
		return
	}
	userData, errCode := strg.storage.GetUserByLogin(authData)
	if errCode != http.StatusOK {
		fmt.Println("Could not get user by login")
		http.Error(w, "Could not get user by login", errCode)
		return
	}
	h := sha256.New()
	h.Write([]byte(authData.Password))
	pswdHash := hex.EncodeToString(h.Sum(nil))
	if pswdHash == userData.Password {
		h := hmac.New(sha256.New, CookieKey)
		h.Write([]byte(userData.UserID))
		sign := h.Sum(nil)
		newCookie := http.Cookie{Name: UserCookie, Value: hex.EncodeToString(append([]byte(userData.UserID)[:], sign[:]...))}
		http.SetCookie(w, &newCookie)
		w.WriteHeader(http.StatusOK)
		w.Write(make([]byte, 0))
	} else {
		fmt.Println("Got wrong login-password pair")
		http.Error(w, "Got wrong login-password pair", http.StatusUnauthorized)
	}
}

func (strg *HandlerWithStorage) AddOrder(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	data, err := io.ReadAll(r.Body)

	if err != nil {
		fmt.Printf("Got error while reading body: ")
	}

	_, errCode := ValidateOrder(string(data))
	if errCode != http.StatusOK {
		fmt.Printf("Got bad order number %s", data)
		http.Error(w, "Got bad order number", errCode)
		return
	}
	userID := r.Context().Value(UserID).(string)
	errCode = strg.storage.AddOrderForUser(string(data), userID)
	if errCode != http.StatusOK || errCode != http.StatusAccepted {
		fmt.Printf("Could not add order into db, %d", errCode)
		http.Error(w, "Could not add order into db", errCode)
		return
	}
	w.WriteHeader(errCode)
	w.Write(make([]byte, 0))
}

func (strg *HandlerWithStorage) GetOrders(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(UserID).(string)
	orders, errCode := strg.storage.GetOrdersByUser(userID)
	if errCode != http.StatusOK {
		http.Error(w, "Got bad status code", errCode)
		return
	}
	if len(orders) == 0 {
		http.Error(w, "Got no orders for this user", http.StatusNoContent)
		return
	}
	ordersMarshalled, err := json.Marshal(orders)
	if err != nil {
		fmt.Printf("Got error: %s", err.Error())
		http.Error(w, "Got error while marshalling", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(ordersMarshalled)
}

func (strg *HandlerWithStorage) GetBalance(w http.ResponseWriter, r *http.Request) {
	userBalance, errCode := strg.storage.GetUserBalance(r.Context().Value(UserID).(string))
	if errCode != http.StatusOK {
		http.Error(w, "Could not get user balance", errCode)
		return
	}
	userBalanceMarshalled, err := json.Marshal(userBalance)
	if err != nil {
		fmt.Printf("Got error while marshalling: %s", err.Error())
		http.Error(w, "Got error while marshalling", errCode)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(userBalanceMarshalled)
}

func (strg *HandlerWithStorage) AddWithdrawal(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(UserID).(string)
	defer r.Body.Close()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		fmt.Printf("Got err %s", err.Error())
		http.Error(w, "Got error while getting data", http.StatusInternalServerError)
		return
	}
	var withdrawal storage.Withdrawal
	err = json.Unmarshal(data, &withdrawal)
	if err != nil {
		fmt.Printf("Got err %s", err.Error())
		http.Error(w, "Got error while getting data", http.StatusInternalServerError)
		return
	}
	_, errCode := ValidateOrder(withdrawal.Order)
	if errCode != http.StatusOK {
		fmt.Printf("Got bad order number %s", withdrawal.Order)
		http.Error(w, "Got bad order number", errCode)
		return
	}
	errCode = strg.storage.AddWithdrawalForUser(userID, withdrawal)
	if errCode != http.StatusOK {
		fmt.Printf("Got errorCode %v", errCode)
		http.Error(w, "Got error from AddWithdrawalForUser", errCode)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(make([]byte, 0))
}

func (strg *HandlerWithStorage) GetWithdrawals(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(UserID).(string)
	withdrawals, errCode := strg.storage.GetWithdrawalsForUser(userID)
	if errCode != http.StatusOK {
		fmt.Printf("Got bad errCode %v", errCode)
		http.Error(w, "Got bad errCode", errCode)
		return
	}
	if len(withdrawals) == 0 {
		http.Error(w, "Got no withdrawals for this user", http.StatusNoContent)
		return
	}
	withdrawalsMarshalled, err := json.Marshal(withdrawals)
	if err != nil {
		fmt.Printf("Got error %s", err.Error())
		http.Error(w, "Got error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(withdrawalsMarshalled)
}

package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/tank4gun/go-musthave-diploma-tpl/internal/mocks"
	"github.com/tank4gun/go-musthave-diploma-tpl/internal/storage"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type wantResponse struct {
	code            int
	headerContent   string
	responseContent string
}

func TestValidateOrder(t *testing.T) {
	tests := []struct {
		name        string
		order       string
		resultOrder uint
		errCode     int
	}{
		{
			name:        "Not integer order",
			order:       "AAA",
			resultOrder: 0,
			errCode:     http.StatusBadRequest,
		},
		{
			name:        "Negative order",
			order:       "-100",
			resultOrder: 0,
			errCode:     http.StatusBadRequest,
		},
		{
			name:        "Correct order with odd digit quantity",
			order:       "133",
			resultOrder: 133,
			errCode:     http.StatusOK,
		},
		{
			name:        "Incorrect order with odd digit quantity",
			order:       "124",
			resultOrder: 0,
			errCode:     http.StatusUnprocessableEntity,
		},
		{
			name:        "Correct order with even digit quantity",
			order:       "5843",
			resultOrder: 5843,
			errCode:     http.StatusOK,
		},
		{
			name:        "Incorrect order with even digit quantity",
			order:       "4723",
			resultOrder: 0,
			errCode:     http.StatusUnprocessableEntity,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, errCode := ValidateOrder(tt.order)
			assert.Equal(t, tt.resultOrder, result)
			assert.Equal(t, tt.errCode, errCode)
		})
	}
}

func TestRegisterHandler(t *testing.T) {
	tt := []struct {
		name                string
		want                wantResponse
		registerData        storage.UserAuthData
		mockResponseID      string
		mockResponseErrCode int
	}{
		{
			"success_register",
			wantResponse{
				http.StatusOK,
				"",
				``,
			},
			storage.UserAuthData{Login: "NewLogin", Password: "MyPassword"},
			"ad29ba3c-7eba-4223-9635-fc71e9c1fa28",
			http.StatusOK,
		},
		{
			"fail_register_same_login",
			wantResponse{
				http.StatusFailedDependency,
				"text/plain; charset=utf-8",
				"Could not register user\n",
			},
			storage.UserAuthData{Login: "NewLogin", Password: "MyPassword"},
			"",
			http.StatusFailedDependency,
		},
		{
			"fail_register_internal_error",
			wantResponse{
				http.StatusInternalServerError,
				"text/plain; charset=utf-8",
				"Could not register user\n",
			},
			storage.UserAuthData{Login: "NewLogin", Password: "MyPassword"},
			"",
			http.StatusInternalServerError,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			marshalledData, _ := json.Marshal(tc.registerData)
			request := httptest.NewRequest(http.MethodGet, "/api/user/register", bytes.NewBuffer(marshalledData))
			w := httptest.NewRecorder()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			storage := mocks.NewMockStorage(ctrl)
			storage.EXPECT().Register(tc.registerData).Return(tc.mockResponseID, tc.mockResponseErrCode)
			handler := http.HandlerFunc(GetHandlerWithStorage(storage).Register)
			handler.ServeHTTP(w, request)
			result := w.Result()
			defer result.Body.Close()
			assert.Equal(t, tc.want.code, result.StatusCode)
			if result.StatusCode == http.StatusOK {
				cookies := result.Cookies()
				for _, cookie := range cookies {
					if cookie.Name == UserCookie {
						h := hmac.New(sha256.New, CookieKey)
						h.Write([]byte(tc.mockResponseID))
						sign := h.Sum(nil)
						assert.Equal(t, hex.EncodeToString(append([]byte(tc.mockResponseID)[:], sign[:]...)), cookie.Value)
						break
					}
					assert.Fail(t, "Got no cookies for UserID")
				}
			}
			responseBody, err := io.ReadAll(result.Body)
			assert.Nil(t, err)
			assert.Equal(t, tc.want.headerContent, result.Header.Get("Content-Type"))
			assert.Equal(t, tc.want.responseContent, string(responseBody))
		})
	}
}

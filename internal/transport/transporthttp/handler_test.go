package transporthttp_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	uuid "github.com/kevinburke/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jeffreyyong/payment-gateway/internal/domain"
	"github.com/jeffreyyong/payment-gateway/internal/transport/transporthttp"
	"github.com/jeffreyyong/payment-gateway/internal/transport/transporthttp/mocks"
)

func TestHandler_Authorize(t *testing.T) {
	requestID, _ := uuid.FromString("79fec15e-a3ea-49b8-989d-6a9ceac77d06")
	var (
		pan                   = "5159640776411853"
		cvv                   = "123"
		expiryMonth           = 1
		expiryYear            = 21
		transactionMinorUnits = uint64(10555)
		mockTransactionID     = uuid.NewV4()
		mockAuthorizationID   = uuid.NewV4()
		authorizationDate     = time.Date(2021, 06, 18, 12, 31, 0, 0, time.UTC)

		authorization = &domain.Authorization{
			RequestID: requestID,
			PaymentSource: domain.PaymentSource{
				PAN: pan,
				CVV: cvv,
				Expiry: domain.Expiry{
					Month: expiryMonth,
					Year:  expiryYear,
				},
			},
			Amount: domain.Amount{
				MinorUnits: transactionMinorUnits,
				Currency:   "GBP",
				Exponent:   2,
			},
			Recipient: domain.Recipient{
				Postcode: "SE17 1FZ",
				LastName: "Yong",
			},
		}

		mockTransaction = &domain.Transaction{
			ID:              mockTransactionID,
			RequestID:       requestID,
			AuthorizationID: mockAuthorizationID,
			AuthorizedAmount: domain.Amount{
				MinorUnits: transactionMinorUnits,
				Currency:   "GBP",
				Exponent:   2,
			},
			CapturedAmount: domain.Amount{
				MinorUnits: 0,
				Currency:   "GBP",
				Exponent:   2,
			},
			RefundedAmount: domain.Amount{
				MinorUnits: 0,
				Currency:   "GBP",
				Exponent:   2,
			},
			PaymentActionSummary: []*domain.PaymentAction{
				{
					Type:          domain.PaymentActionTypeAuthorization,
					Status:        domain.PaymentActionStatusSuccess,
					ProcessedDate: authorizationDate,
					Amount: &domain.Amount{
						MinorUnits: transactionMinorUnits,
						Currency:   "GBP",
						Exponent:   2,
					},
					RequestID: requestID,
				},
			},
		}

		mockTransactionWithNoAuthorizationDate = &domain.Transaction{
			ID:              mockTransactionID,
			RequestID:       requestID,
			AuthorizationID: mockAuthorizationID,
			AuthorizedAmount: domain.Amount{
				MinorUnits: transactionMinorUnits,
				Currency:   "GBP",
				Exponent:   2,
			},
			CapturedAmount: domain.Amount{
				MinorUnits: 0,
				Currency:   "GBP",
				Exponent:   2,
			},
			RefundedAmount: domain.Amount{
				MinorUnits: 0,
				Currency:   "GBP",
				Exponent:   2,
			},
			PaymentActionSummary: []*domain.PaymentAction{},
		}

		validReqBody = `
	{
		"request_id": "79fec15e-a3ea-49b8-989d-6a9ceac77d06",
		"payment_source": {
			"pan": "5159640776411853",
			"cvv": "123",
			"expiry_month": 1,
			"expiry_year": 21
		},
		"amount": {
			"minor_units": 10555,
			"currency": "GBP",
			"exponent": 2
		},
		"description": "APPLE.COM",
		"recipient": {
			"postcode": "SE17 1FZ",
			"last_name": "Yong"
		}
	}`

		expectedTransactionResp = transporthttp.Transaction{
			ID:              mockTransactionID,
			AuthorizationID: mockAuthorizationID,
			AuthorizedTime:  &authorizationDate,
			AuthorizedAmount: transporthttp.Amount{
				MinorUnits: mockTransaction.AuthorizedAmount.MinorUnits,
				Exponent:   mockTransaction.AuthorizedAmount.Exponent,
				Currency:   mockTransaction.AuthorizedAmount.Currency,
			},
			CapturedAmount: transporthttp.Amount{
				MinorUnits: mockTransaction.CapturedAmount.MinorUnits,
				Exponent:   mockTransaction.CapturedAmount.Exponent,
				Currency:   mockTransaction.CapturedAmount.Currency,
			},
			RefundedAmount: transporthttp.Amount{
				MinorUnits: mockTransaction.RefundedAmount.MinorUnits,
				Exponent:   mockTransaction.RefundedAmount.Exponent,
				Currency:   mockTransaction.RefundedAmount.Currency,
			},
			IsVoided: false,
		}
	)
	t.Run("SUCCESS", func(t *testing.T) {
		t.Run("should authorize the transaction, return status code 200", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			srv := mocks.NewMockService(ctrl)
			srv.EXPECT().Authorize(gomock.Any(), authorization).Return(mockTransaction, nil)

			h, err := transporthttp.NewHTTPHandler(srv)
			require.NoError(t, err)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(
				http.MethodPost,
				transporthttp.EndpointAuthorize,
				bytes.NewReader([]byte(validReqBody)),
			)

			h.Authorize(w, r)
			res := w.Result()
			defer res.Body.Close()
			assert.Equal(t, http.StatusOK, res.StatusCode)
			assert.Equal(t, transporthttp.ApplicationJSON, res.Header.Get(transporthttp.ContentType))

			var out transporthttp.Transaction
			require.NoError(t, json.NewDecoder(res.Body).Decode(&out))
			assert.Equal(t, expectedTransactionResp, out)
		})
	})

	t.Run("FAILURE", func(t *testing.T) {
		type handlerMocks struct {
			service *mocks.MockService
		}

		failureCases := []struct {
			description          string
			requestBody          io.Reader
			setupMocks           func(m *handlerMocks)
			expectedStatusCode   int
			expectedResponseBody string
		}{
			{
				"no request body is provided",
				nil,
				nil,
				http.StatusBadRequest,
				`{"code":"bad_request","message":"missing request body"}`,
			},
			{
				"malformed json request body",
				bytes.NewReader([]byte(`{`)),
				nil,
				http.StatusBadRequest,
				`{"code":"bad_request","message":"failed to unmarshal request body"}`,
			},
			{
				"service returns error",
				bytes.NewReader([]byte(validReqBody)),
				func(m *handlerMocks) {
					m.service.EXPECT().Authorize(gomock.Any(), authorization).Return(nil, errors.New("kaboom"))
				},
				http.StatusInternalServerError,
				`{"code":"unknown_failure","message":"failed to authorize transaction in service"}`,
			},
			{
				"transaction has no authorization date",
				bytes.NewReader([]byte(validReqBody)),
				func(m *handlerMocks) {
					m.service.EXPECT().Authorize(gomock.Any(), authorization).Return(mockTransactionWithNoAuthorizationDate, nil)
				},
				http.StatusInternalServerError,
				`{"code":"unknown_failure","message":"invalid transaction with no authorization date"}`,
			},
		}

		for _, tt := range failureCases {
			t.Run(tt.description, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()
				srv := mocks.NewMockService(ctrl)

				m := handlerMocks{service: srv}
				if tt.setupMocks != nil {
					tt.setupMocks(&m)
				}

				w := httptest.NewRecorder()
				r := httptest.NewRequest(
					http.MethodPost,
					transporthttp.EndpointAuthorize,
					tt.requestBody,
				)

				h, err := transporthttp.NewHTTPHandler(srv)
				require.NoError(t, err)

				h.Authorize(w, r)
				res := w.Result()
				defer res.Body.Close()
				assert.Equal(t, tt.expectedStatusCode, res.StatusCode)
				assert.Equal(t, transporthttp.ApplicationJSON, res.Header.Get(transporthttp.ContentType))

				respBody, err := ioutil.ReadAll(res.Body)
				require.NoError(t, err)
				assert.Equal(t, tt.expectedResponseBody, strings.TrimSuffix(string(respBody), "\n"))
			})
		}
	})
}

func TestHandler_Void(t *testing.T) {
	requestID, _ := uuid.FromString("79fec15e-a3ea-49b8-989d-6a9ceac77d06")
	someAuthorizationID, _ := uuid.FromString("f71d1314-2fbb-44cc-ba27-527c6682e3a5")
	var (
		transactionMinorUnits = uint64(10555)
		mockTransactionID     = uuid.NewV4()
		mockAuthorizationID   = uuid.NewV4()
		authorizationDate     = time.Date(2021, 06, 18, 12, 31, 0, 0, time.UTC)

		void = &domain.Void{
			RequestID:       requestID,
			AuthorizationID: someAuthorizationID,
		}

		mockTransaction = &domain.Transaction{
			ID:              mockTransactionID,
			RequestID:       requestID,
			AuthorizationID: mockAuthorizationID,
			AuthorizedAmount: domain.Amount{
				MinorUnits: transactionMinorUnits,
				Currency:   "GBP",
				Exponent:   2,
			},
			CapturedAmount: domain.Amount{
				MinorUnits: 0,
				Currency:   "GBP",
				Exponent:   2,
			},
			RefundedAmount: domain.Amount{
				MinorUnits: 0,
				Currency:   "GBP",
				Exponent:   2,
			},
			PaymentActionSummary: []*domain.PaymentAction{
				{
					Type:          domain.PaymentActionTypeAuthorization,
					Status:        domain.PaymentActionStatusSuccess,
					ProcessedDate: authorizationDate,
					Amount: &domain.Amount{
						MinorUnits: transactionMinorUnits,
						Currency:   "GBP",
						Exponent:   2,
					},
					RequestID: requestID,
				},
			},
		}

		mockTransactionWithNoAuthorizationDate = &domain.Transaction{
			ID:              mockTransactionID,
			RequestID:       requestID,
			AuthorizationID: mockAuthorizationID,
			AuthorizedAmount: domain.Amount{
				MinorUnits: transactionMinorUnits,
				Currency:   "GBP",
				Exponent:   2,
			},
			CapturedAmount: domain.Amount{
				MinorUnits: 0,
				Currency:   "GBP",
				Exponent:   2,
			},
			RefundedAmount: domain.Amount{
				MinorUnits: 0,
				Currency:   "GBP",
				Exponent:   2,
			},
			PaymentActionSummary: []*domain.PaymentAction{},
		}

		validReqBody = `
		{
			"request_id": "79fec15e-a3ea-49b8-989d-6a9ceac77d06",
			"authorization_id": "f71d1314-2fbb-44cc-ba27-527c6682e3a5"
		}`

		expectedTransactionResp = transporthttp.Transaction{
			ID:              mockTransactionID,
			AuthorizationID: mockAuthorizationID,
			AuthorizedTime:  &authorizationDate,
			AuthorizedAmount: transporthttp.Amount{
				MinorUnits: mockTransaction.AuthorizedAmount.MinorUnits,
				Exponent:   mockTransaction.AuthorizedAmount.Exponent,
				Currency:   mockTransaction.AuthorizedAmount.Currency,
			},
			CapturedAmount: transporthttp.Amount{
				MinorUnits: mockTransaction.CapturedAmount.MinorUnits,
				Exponent:   mockTransaction.CapturedAmount.Exponent,
				Currency:   mockTransaction.CapturedAmount.Currency,
			},
			RefundedAmount: transporthttp.Amount{
				MinorUnits: mockTransaction.RefundedAmount.MinorUnits,
				Exponent:   mockTransaction.RefundedAmount.Exponent,
				Currency:   mockTransaction.RefundedAmount.Currency,
			},
			IsVoided: false,
		}
	)
	t.Run("SUCCESS", func(t *testing.T) {
		t.Run("should void the transaction, return status code 200", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			srv := mocks.NewMockService(ctrl)
			srv.EXPECT().Void(gomock.Any(), void).Return(mockTransaction, nil)

			h, err := transporthttp.NewHTTPHandler(srv)
			require.NoError(t, err)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(
				http.MethodPost,
				transporthttp.EndpointVoid,
				bytes.NewReader([]byte(validReqBody)),
			)

			h.Void(w, r)
			res := w.Result()
			defer res.Body.Close()
			assert.Equal(t, http.StatusOK, res.StatusCode)
			assert.Equal(t, transporthttp.ApplicationJSON, res.Header.Get(transporthttp.ContentType))

			var out transporthttp.Transaction
			require.NoError(t, json.NewDecoder(res.Body).Decode(&out))
			assert.Equal(t, expectedTransactionResp, out)
		})
	})

	t.Run("FAILURE", func(t *testing.T) {
		type handlerMocks struct {
			service *mocks.MockService
		}

		failureCases := []struct {
			description          string
			requestBody          io.Reader
			setupMocks           func(m *handlerMocks)
			expectedStatusCode   int
			expectedResponseBody string
		}{
			{
				"no request body is provided",
				nil,
				nil,
				http.StatusBadRequest,
				`{"code":"bad_request","message":"missing request body"}`,
			},
			{
				"malformed json request body",
				bytes.NewReader([]byte(`{`)),
				nil,
				http.StatusBadRequest,
				`{"code":"bad_request","message":"failed to unmarshal request body"}`,
			},
			{
				"service returns error",
				bytes.NewReader([]byte(validReqBody)),
				func(m *handlerMocks) {
					m.service.EXPECT().Void(gomock.Any(), void).Return(nil, errors.New("kaboom"))
				},
				http.StatusInternalServerError,
				`{"code":"unknown_failure","message":"failed to void transaction in service"}`,
			},
			{
				"transaction has no authorization date",
				bytes.NewReader([]byte(validReqBody)),
				func(m *handlerMocks) {
					m.service.EXPECT().Void(gomock.Any(), void).Return(mockTransactionWithNoAuthorizationDate, nil)
				},
				http.StatusInternalServerError,
				`{"code":"unknown_failure","message":"invalid transaction with no authorization date"}`,
			},
			{
				"service returns unprocessable error",
				bytes.NewReader([]byte(validReqBody)),
				func(m *handlerMocks) {
					m.service.EXPECT().Void(gomock.Any(), void).Return(nil, domain.ErrUnprocessable)
				},
				http.StatusUnprocessableEntity,
				`{"code":"unprocessable","message":"unprocessable"}`,
			},
		}

		for _, tt := range failureCases {
			t.Run(tt.description, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()
				srv := mocks.NewMockService(ctrl)

				m := handlerMocks{service: srv}
				if tt.setupMocks != nil {
					tt.setupMocks(&m)
				}

				w := httptest.NewRecorder()
				r := httptest.NewRequest(
					http.MethodPost,
					transporthttp.EndpointVoid,
					tt.requestBody,
				)

				h, err := transporthttp.NewHTTPHandler(srv)
				require.NoError(t, err)

				h.Void(w, r)
				res := w.Result()
				defer res.Body.Close()
				assert.Equal(t, tt.expectedStatusCode, res.StatusCode)
				assert.Equal(t, transporthttp.ApplicationJSON, res.Header.Get(transporthttp.ContentType))

				respBody, err := ioutil.ReadAll(res.Body)
				require.NoError(t, err)
				assert.Equal(t, tt.expectedResponseBody, strings.TrimSuffix(string(respBody), "\n"))
			})
		}
	})
}

func TestHandler_Capture(t *testing.T) {
	captureRequestID, _ := uuid.FromString("cf533318-ed57-411e-be6a-f74b032d594f")
	someAuthorizationID, _ := uuid.FromString("f71d1314-2fbb-44cc-ba27-527c6682e3a5")
	var (
		requestID             = uuid.NewV4()
		transactionMinorUnits = uint64(10555)
		captureMinorUnits     = uint64(5555)
		mockTransactionID     = uuid.NewV4()
		mockAuthorizationID   = uuid.NewV4()
		authorizationDate     = time.Date(2021, 06, 18, 12, 31, 0, 0, time.UTC)
		captureDate           = authorizationDate.Add(1 * time.Hour)

		capture = &domain.Capture{
			RequestID:       captureRequestID,
			AuthorizationID: someAuthorizationID,
			Amount: domain.Amount{
				MinorUnits: captureMinorUnits,
				Currency:   "GBP",
				Exponent:   2,
			},
		}

		mockTransaction = &domain.Transaction{
			ID:              mockTransactionID,
			RequestID:       requestID,
			AuthorizationID: mockAuthorizationID,
			AuthorizedAmount: domain.Amount{
				MinorUnits: transactionMinorUnits,
				Currency:   "GBP",
				Exponent:   2,
			},
			CapturedAmount: domain.Amount{
				MinorUnits: captureMinorUnits,
				Currency:   "GBP",
				Exponent:   2,
			},
			RefundedAmount: domain.Amount{
				MinorUnits: 0,
				Currency:   "GBP",
				Exponent:   2,
			},
			PaymentActionSummary: []*domain.PaymentAction{
				{
					Type:          domain.PaymentActionTypeAuthorization,
					Status:        domain.PaymentActionStatusSuccess,
					ProcessedDate: authorizationDate,
					Amount: &domain.Amount{
						MinorUnits: transactionMinorUnits,
						Currency:   "GBP",
						Exponent:   2,
					},
					RequestID: requestID,
				},
				{
					Type:          domain.PaymentActionTypeCapture,
					Status:        domain.PaymentActionStatusSuccess,
					ProcessedDate: captureDate,
					Amount: &domain.Amount{
						MinorUnits: captureMinorUnits,
						Currency:   "GBP",
						Exponent:   2,
					},
					RequestID: captureRequestID,
				},
			},
		}

		validReqBody = `
			{
				"request_id": "cf533318-ed57-411e-be6a-f74b032d594f",
				"amount": {
					"minor_units": 5555,
					"currency": "GBP",
					"exponent": 2
				},
				"authorization_id": "f71d1314-2fbb-44cc-ba27-527c6682e3a5"
			}`

		expectedTransactionResp = transporthttp.Transaction{
			ID:              mockTransactionID,
			AuthorizationID: mockAuthorizationID,
			AuthorizedTime:  &authorizationDate,
			AuthorizedAmount: transporthttp.Amount{
				MinorUnits: mockTransaction.AuthorizedAmount.MinorUnits,
				Exponent:   mockTransaction.AuthorizedAmount.Exponent,
				Currency:   mockTransaction.AuthorizedAmount.Currency,
			},
			CapturedAmount: transporthttp.Amount{
				MinorUnits: mockTransaction.CapturedAmount.MinorUnits,
				Exponent:   mockTransaction.CapturedAmount.Exponent,
				Currency:   mockTransaction.CapturedAmount.Currency,
			},
			RefundedAmount: transporthttp.Amount{
				MinorUnits: mockTransaction.RefundedAmount.MinorUnits,
				Exponent:   mockTransaction.RefundedAmount.Exponent,
				Currency:   mockTransaction.RefundedAmount.Currency,
			},
			IsVoided: false,
		}
	)
	t.Run("SUCCESS", func(t *testing.T) {
		t.Run("should capture the transaction, return status code 200", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			srv := mocks.NewMockService(ctrl)
			srv.EXPECT().Capture(gomock.Any(), capture).Return(mockTransaction, nil)

			h, err := transporthttp.NewHTTPHandler(srv)
			require.NoError(t, err)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(
				http.MethodPost,
				transporthttp.EndpointCapture,
				bytes.NewReader([]byte(validReqBody)),
			)

			h.Capture(w, r)
			res := w.Result()
			defer res.Body.Close()
			assert.Equal(t, http.StatusOK, res.StatusCode)
			assert.Equal(t, transporthttp.ApplicationJSON, res.Header.Get(transporthttp.ContentType))

			var out transporthttp.Transaction
			require.NoError(t, json.NewDecoder(res.Body).Decode(&out))
			assert.Equal(t, expectedTransactionResp, out)
		})
	})

	t.Run("FAILURE", func(t *testing.T) {
		type handlerMocks struct {
			service *mocks.MockService
		}

		failureCases := []struct {
			description          string
			requestBody          io.Reader
			setupMocks           func(m *handlerMocks)
			expectedStatusCode   int
			expectedResponseBody string
		}{
			{
				"no request body is provided",
				nil,
				nil,
				http.StatusBadRequest,
				`{"code":"bad_request","message":"missing request body"}`,
			},
			{
				"malformed json request body",
				bytes.NewReader([]byte(`{`)),
				nil,
				http.StatusBadRequest,
				`{"code":"bad_request","message":"failed to unmarshal request body"}`,
			},
			{
				"service returns error",
				bytes.NewReader([]byte(validReqBody)),
				func(m *handlerMocks) {
					m.service.EXPECT().Capture(gomock.Any(), capture).Return(nil, errors.New("kaboom"))
				},
				http.StatusInternalServerError,
				`{"code":"unknown_failure","message":"failed to capture transaction in service"}`,
			},
			{
				"service returns transaction not found",
				bytes.NewReader([]byte(validReqBody)),
				func(m *handlerMocks) {
					m.service.EXPECT().Capture(gomock.Any(), capture).Return(nil, domain.ErrTransactionNotFound)
				},
				http.StatusNotFound,
				`{"code":"not_found","message":"unable to find the transaction with the authorization ID"}`,
			},
			{
				"service returns unprocessable error",
				bytes.NewReader([]byte(validReqBody)),
				func(m *handlerMocks) {
					m.service.EXPECT().Capture(gomock.Any(), capture).Return(nil, domain.ErrUnprocessable)
				},
				http.StatusUnprocessableEntity,
				`{"code":"unprocessable","message":"unprocessable"}`,
			},
		}

		for _, tt := range failureCases {
			t.Run(tt.description, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()
				srv := mocks.NewMockService(ctrl)

				m := handlerMocks{service: srv}
				if tt.setupMocks != nil {
					tt.setupMocks(&m)
				}

				w := httptest.NewRecorder()
				r := httptest.NewRequest(
					http.MethodPost,
					transporthttp.EndpointCapture,
					tt.requestBody,
				)

				h, err := transporthttp.NewHTTPHandler(srv)
				require.NoError(t, err)

				h.Capture(w, r)
				res := w.Result()
				defer res.Body.Close()
				assert.Equal(t, tt.expectedStatusCode, res.StatusCode)
				assert.Equal(t, transporthttp.ApplicationJSON, res.Header.Get(transporthttp.ContentType))

				respBody, err := ioutil.ReadAll(res.Body)
				require.NoError(t, err)
				assert.Equal(t, tt.expectedResponseBody, strings.TrimSuffix(string(respBody), "\n"))
			})
		}
	})
}

func TestHandler_Refund(t *testing.T) {
	refundRequestID, _ := uuid.FromString("cf533318-ed57-411e-be6a-f74b032d594f")
	someAuthorizationID, _ := uuid.FromString("f71d1314-2fbb-44cc-ba27-527c6682e3a5")
	var (
		requestID             = uuid.NewV4()
		transactionMinorUnits = uint64(10555)
		captureMinorUnits     = uint64(5555)
		refundMinorUnits      = uint64(5555)
		mockTransactionID     = uuid.NewV4()
		mockAuthorizationID   = uuid.NewV4()
		authorizationDate     = time.Date(2021, 06, 18, 12, 31, 0, 0, time.UTC)
		captureDate           = authorizationDate.Add(1 * time.Hour)
		refundDate            = captureDate.Add(1 * time.Hour)

		refund = &domain.Refund{
			RequestID:       refundRequestID,
			AuthorizationID: someAuthorizationID,
			Amount: domain.Amount{
				MinorUnits: refundMinorUnits,
				Currency:   "GBP",
				Exponent:   2,
			},
		}

		mockTransaction = &domain.Transaction{
			ID:              mockTransactionID,
			RequestID:       requestID,
			AuthorizationID: mockAuthorizationID,
			AuthorizedAmount: domain.Amount{
				MinorUnits: transactionMinorUnits,
				Currency:   "GBP",
				Exponent:   2,
			},
			CapturedAmount: domain.Amount{
				MinorUnits: captureMinorUnits,
				Currency:   "GBP",
				Exponent:   2,
			},
			RefundedAmount: domain.Amount{
				MinorUnits: refundMinorUnits,
				Currency:   "GBP",
				Exponent:   2,
			},
			PaymentActionSummary: []*domain.PaymentAction{
				{
					Type:          domain.PaymentActionTypeAuthorization,
					Status:        domain.PaymentActionStatusSuccess,
					ProcessedDate: authorizationDate,
					Amount: &domain.Amount{
						MinorUnits: transactionMinorUnits,
						Currency:   "GBP",
						Exponent:   2,
					},
					RequestID: requestID,
				},
				{
					Type:          domain.PaymentActionTypeCapture,
					Status:        domain.PaymentActionStatusSuccess,
					ProcessedDate: captureDate,
					Amount: &domain.Amount{
						MinorUnits: captureMinorUnits,
						Currency:   "GBP",
						Exponent:   2,
					},
					RequestID: requestID,
				},
				{
					Type:          domain.PaymentActionTypeRefund,
					Status:        domain.PaymentActionStatusSuccess,
					ProcessedDate: refundDate,
					Amount: &domain.Amount{
						MinorUnits: refundMinorUnits,
						Currency:   "GBP",
						Exponent:   2,
					},
					RequestID: refundRequestID,
				},
			},
		}

		validReqBody = `
			{
				"request_id": "cf533318-ed57-411e-be6a-f74b032d594f",
				"amount": {
					"minor_units": 5555,
					"currency": "GBP",
					"exponent": 2
				},
				"authorization_id": "f71d1314-2fbb-44cc-ba27-527c6682e3a5"
			}`

		expectedTransactionResp = transporthttp.Transaction{
			ID:              mockTransactionID,
			AuthorizationID: mockAuthorizationID,
			AuthorizedTime:  &authorizationDate,
			AuthorizedAmount: transporthttp.Amount{
				MinorUnits: mockTransaction.AuthorizedAmount.MinorUnits,
				Exponent:   mockTransaction.AuthorizedAmount.Exponent,
				Currency:   mockTransaction.AuthorizedAmount.Currency,
			},
			CapturedAmount: transporthttp.Amount{
				MinorUnits: mockTransaction.CapturedAmount.MinorUnits,
				Exponent:   mockTransaction.CapturedAmount.Exponent,
				Currency:   mockTransaction.CapturedAmount.Currency,
			},
			RefundedAmount: transporthttp.Amount{
				MinorUnits: mockTransaction.RefundedAmount.MinorUnits,
				Exponent:   mockTransaction.RefundedAmount.Exponent,
				Currency:   mockTransaction.RefundedAmount.Currency,
			},
			IsVoided: false,
		}
	)
	t.Run("SUCCESS", func(t *testing.T) {
		t.Run("should refund the transaction, return status code 200", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			srv := mocks.NewMockService(ctrl)
			srv.EXPECT().Refund(gomock.Any(), refund).Return(mockTransaction, nil)

			h, err := transporthttp.NewHTTPHandler(srv)
			require.NoError(t, err)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(
				http.MethodPost,
				transporthttp.EndpointRefund,
				bytes.NewReader([]byte(validReqBody)),
			)

			h.Refund(w, r)
			res := w.Result()
			defer res.Body.Close()
			assert.Equal(t, http.StatusOK, res.StatusCode)
			assert.Equal(t, transporthttp.ApplicationJSON, res.Header.Get(transporthttp.ContentType))

			var out transporthttp.Transaction
			require.NoError(t, json.NewDecoder(res.Body).Decode(&out))
			assert.Equal(t, expectedTransactionResp, out)
		})
	})

	t.Run("FAILURE", func(t *testing.T) {
		type handlerMocks struct {
			service *mocks.MockService
		}

		failureCases := []struct {
			description          string
			requestBody          io.Reader
			setupMocks           func(m *handlerMocks)
			expectedStatusCode   int
			expectedResponseBody string
		}{
			{
				"no request body is provided",
				nil,
				nil,
				http.StatusBadRequest,
				`{"code":"bad_request","message":"missing request body"}`,
			},
			{
				"malformed json request body",
				bytes.NewReader([]byte(`{`)),
				nil,
				http.StatusBadRequest,
				`{"code":"bad_request","message":"failed to unmarshal request body"}`,
			},
			{
				"service returns error",
				bytes.NewReader([]byte(validReqBody)),
				func(m *handlerMocks) {
					m.service.EXPECT().Refund(gomock.Any(), refund).Return(nil, errors.New("kaboom"))
				},
				http.StatusInternalServerError,
				`{"code":"unknown_failure","message":"failed to refund transaction in service"}`,
			},
			{
				"service returns transaction not found",
				bytes.NewReader([]byte(validReqBody)),
				func(m *handlerMocks) {
					m.service.EXPECT().Refund(gomock.Any(), refund).Return(nil, domain.ErrTransactionNotFound)
				},
				http.StatusNotFound,
				`{"code":"not_found","message":"unable to find the transaction with the authorization ID"}`,
			},
			{
				"service returns unprocessable error",
				bytes.NewReader([]byte(validReqBody)),
				func(m *handlerMocks) {
					m.service.EXPECT().Refund(gomock.Any(), refund).Return(nil, domain.ErrUnprocessable)
				},
				http.StatusUnprocessableEntity,
				`{"code":"unprocessable","message":"unprocessable"}`,
			},
		}

		for _, tt := range failureCases {
			t.Run(tt.description, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()
				srv := mocks.NewMockService(ctrl)

				m := handlerMocks{service: srv}
				if tt.setupMocks != nil {
					tt.setupMocks(&m)
				}

				w := httptest.NewRecorder()
				r := httptest.NewRequest(
					http.MethodPost,
					transporthttp.EndpointRefund,
					tt.requestBody,
				)

				h, err := transporthttp.NewHTTPHandler(srv)
				require.NoError(t, err)

				h.Refund(w, r)
				res := w.Result()
				defer res.Body.Close()
				assert.Equal(t, tt.expectedStatusCode, res.StatusCode)
				assert.Equal(t, transporthttp.ApplicationJSON, res.Header.Get(transporthttp.ContentType))

				respBody, err := ioutil.ReadAll(res.Body)
				require.NoError(t, err)
				assert.Equal(t, tt.expectedResponseBody, strings.TrimSuffix(string(respBody), "\n"))
			})
		}
	})
}

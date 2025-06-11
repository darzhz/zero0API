package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"time"
	"zero0Api/utils"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func SetupPaymentsRoutes(app *pocketbase.PocketBase) func(*core.ServeEvent) error {
	return func(e *core.ServeEvent) error {
		e.Router.POST("/api/initiate-payment", func(c *core.RequestEvent) error {
			token, err := utils.GetPGAuthToken()
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]any{
					"error": "Unable to fetch auth token",
				})
			}

			// Optional: Read payment input (like amount, customer info) from the request
			// For now, hardcoding an example payment body
			paymentPayload := utils.PaymentRequest{
				MerchantOrderID: "order12345",
				Amount:          1,
				ExpireAfter:     1200,
				MetaInfo: utils.MetaInfo{
					UDF1: "example_udf1",
					UDF2: "example_udf2",
					UDF3: "example_udf3",
					UDF4: "example_udf4",
					UDF5: "example_udf5",
				},
				PaymentFlow: utils.FlowInfo{
					Type:    "PG_CHECKOUT",
					Message: "Please complete your payment",
					MerchantUrls: utils.MerchantURLs{
						RedirectURL: "https://example.com/payment-success",
					},
				},
			}
			payloadBytes, _ := json.Marshal(paymentPayload)

			req, _ := http.NewRequest("POST", os.Getenv("PG_PAYMENT_CREATOR_API"), bytes.NewBuffer(payloadBytes))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "O-Bearer "+token)

			client := http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil || resp.StatusCode >= 400 {
				return c.JSON(http.StatusBadGateway, map[string]any{
					"error": "PG call failed",
				})
			}
			defer resp.Body.Close()

			var pgResponse map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&pgResponse); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]any{
					"error": "Failed to decode PG response",
				})
			}

			return c.JSON(http.StatusOK, pgResponse)
		})

		return e.Next()
	}
}

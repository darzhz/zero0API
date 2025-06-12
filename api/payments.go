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
			var input utils.PaymentRequestClient
			if err := json.NewDecoder(c.Request.Body).Decode(&input); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{
					"error": "Invalid or missing request body: " + err.Error(),
				})
			}

			MerchantOrderID := utils.GenerateRandomUUID()

			// print input values for debugging
			utils.LogDebug("Payment Input", map[string]any{
				"amount":     input.Amount,
				"entityType": input.EntityType,
				"entityId":   input.EntityID,
				"vendorId":   input.VendorID,
				"userId":     input.UserID,
			})
			paymentPayload := utils.PaymentRequest{
				MerchantOrderID: MerchantOrderID,
				Amount:          input.Amount,
				ExpireAfter:     3000,
				MetaInfo: utils.MetaInfo{
					UDF1: input.EntityType,
					UDF2: input.EntityID,
					UDF3: input.VendorID,
					UDF4: input.UserID,
				},
				PaymentFlow: utils.FlowInfo{
					Type:    "PG_CHECKOUT",
					Message: "Please complete your payment",
					MerchantUrls: utils.MerchantURLs{
						RedirectURL: "https://zero0.cutify.space/",
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
			collection, err := app.FindCollectionByNameOrId("payments")
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]any{
					"error": "Unable to fetch collection",
				})
			}
			record := core.NewRecord(collection)
			record.Set("merchantOrderID", MerchantOrderID)
			record.Set("amount", input.Amount)
			record.Set("entityType", input.EntityType)
			record.Set("entityID", input.EntityID)
			record.Set("vendorID", input.VendorID)
			record.Set("status", "PENDING")
			record.Set("userID", input.UserID)
			record.Set("orderID", pgResponse["orderId"].(string))
			record.Set("expireAt", int(pgResponse["expireAt"].(float64)))
			expireAt := int64(pgResponse["expireAt"].(float64))

			err = app.Save(record)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]any{
					"error": "Unable to save payment record",
				})
			}

			utils.AddOrderJob(MerchantOrderID, expireAt)
			return c.JSON(http.StatusOK, pgResponse)
		})

		return e.Next()
	}
}

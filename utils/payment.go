package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type AuthPayload struct {
	ClientID      string `json:"client_id"`
	ClientVersion string `json:"client_version"`
	ClientSecret  string `json:"client_secret"`
	GrantType     string `json:"grant_type"`
}

type PaymentRequest struct {
	MerchantOrderID string   `json:"merchantOrderId"`
	Amount          int      `json:"amount"`
	ExpireAfter     int      `json:"expireAfter"`
	MetaInfo        MetaInfo `json:"metaInfo"`
	PaymentFlow     FlowInfo `json:"paymentFlow"`
}

type MetaInfo struct {
	UDF1 string `json:"udf1"`
	UDF2 string `json:"udf2"`
	UDF3 string `json:"udf3"`
	UDF4 string `json:"udf4"`
	UDF5 string `json:"udf5"`
}

type FlowInfo struct {
	Type         string       `json:"type"`
	Message      string       `json:"message"`
	MerchantUrls MerchantURLs `json:"merchantUrls"`
}

type MerchantURLs struct {
	RedirectURL string `json:"redirectUrl"`
}

var tokenCache struct {
	sync.Mutex
	Token     string
	ExpiresAt time.Time
}

// Replace this with actual PG auth endpoint & credentials
func GetPGAuthToken() (string, error) {
	tokenCache.Lock()
	defer tokenCache.Unlock()

	buffer := time.Minute
	if tokenCache.Token != "" && time.Now().Before(tokenCache.ExpiresAt.Add(-buffer)) {
		return tokenCache.Token, nil
	}

	// Example auth body
	authPayload := AuthPayload{
		ClientID:      os.Getenv("PG_CLIENT_ID"),
		ClientVersion: os.Getenv("PG_CLIENT_VERSION"),
		ClientSecret:  os.Getenv("PG_CLIENT_SECRET"),
		GrantType:     "client_credentials",
	}
	data := url.Values{}
	data.Set("client_id", authPayload.ClientID)
	data.Set("client_version", authPayload.ClientVersion)
	data.Set("client_secret", authPayload.ClientSecret)
	data.Set("grant_type", authPayload.GrantType)

	payloadBytes, _ := json.Marshal(authPayload)
	fmt.Println("auth payload:", string(payloadBytes))

	req, err := http.NewRequest("POST", os.Getenv("PG_AUTH_URL"), strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	fmt.Println("request sent to PG auth URL:", os.Getenv("PG_AUTH_URL"))
	if err != nil {
		return "", err
	}
	fmt.Println("response received from PG auth URL:", os.Getenv("PG_AUTH_URL"))
	if resp == nil {
		return "", err
	}
	fmt.Println("response status code:", resp.StatusCode)
	fmt.Println("response status:", resp.Status)

	defer resp.Body.Close()

	var AuthResp struct {
		AccessToken          string `json:"access_token"`
		EncryptedAccessToken string `json:"encrypted_access_token"`
		ExpiresIn            int    `json:"expires_in"`
		IssuedAt             int64  `json:"issued_at"`
		ExpiresAt            int64  `json:"expires_at"`
		SessionExpiresAt     int64  `json:"session_expires_at"`
		TokenType            string `json:"token_type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&AuthResp); err != nil {
		log.Println("Error decoding PG auth response:", err)
		return "", err
	}

	tokenCache.Token = AuthResp.AccessToken
	tokenCache.ExpiresAt = time.Now().Add(time.Duration(AuthResp.ExpiresIn) * time.Second)
	log.Println("PG auth token fetched successfully:", tokenCache.Token)
	return tokenCache.Token, nil
}

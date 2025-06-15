package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase"
)

type OrderJob struct {
	MerchantOrderID string
	Expires         time.Time
	State           string
	NextPoll        time.Time
	Interval        time.Duration
}
type OrderResponse struct {
	OrderID           string          `json:"orderId"`
	State             string          `json:"state"`
	Amount            int             `json:"amount"`
	ExpireAt          int64           `json:"expireAt"`
	ErrorCode         string          `json:"errorCode"`
	DetailedErrorCode string          `json:"detailedErrorCode"`
	MetaInfo          MetaInfo        `json:"metaInfo"`
	PaymentDetails    []PaymentDetail `json:"paymentDetails"`
}

type PaymentDetail struct {
	PaymentMode       string            `json:"paymentMode"`
	TransactionID     string            `json:"transactionId"`
	Timestamp         int64             `json:"timestamp"`
	Amount            int               `json:"amount"`
	State             string            `json:"state"`
	ErrorCode         string            `json:"errorCode"`
	DetailedErrorCode string            `json:"detailedErrorCode"`
	SplitInstruments  []SplitInstrument `json:"splitInstruments"`
}

type SplitInstrument struct {
	Amount     int        `json:"amount"`
	Rail       Rail       `json:"rail"`
	Instrument Instrument `json:"instrument"`
}

type Rail struct {
	Type             string `json:"type"`
	UpiTransactionID string `json:"upiTransactionId"`
}

type Instrument struct {
	Type                string `json:"type"`
	MaskedAccountNumber string `json:"maskedAccountNumber"`
	AccountType         string `json:"accountType"`
	AccountHolderName   string `json:"accountHolderName"`
	IFSC                string `json:"ifsc"`
}

var jobQueue = struct {
	sync.Mutex
	jobs []*OrderJob
}{jobs: []*OrderJob{}}

func AddOrderJob(id string, expireAtMillis int64) {
	expireAt := time.UnixMilli(expireAtMillis)
	fmt.Println("Adding order job:", id, "which expires at", expireAt)
	if time.Now().After(expireAt) {
		fmt.Println("Job already expired, skipping:", id)
		return
	}

	job := &OrderJob{
		MerchantOrderID: id,
		Expires:         expireAt,
		NextPoll:        time.Now().Add(3 * time.Second), // poll soon
		Interval:        3 * time.Second,
	}

	jobQueue.Lock()
	jobQueue.jobs = append(jobQueue.jobs, job)
	jobQueue.Unlock()
}

func getOrderStatus(app *pocketbase.PocketBase, merchantOrderID string) (*OrderJob, error) {
	fmt.Println("Attempting to fetch order status for", merchantOrderID)
	token, err := GetPGAuthToken()
	if err != nil {
		return nil, err
	}
	//Checking if the order is completed or failed in db
	record, err := app.FindFirstRecordByFilter("payments", fmt.Sprintf("merchantOrderID = '%s'", merchantOrderID))
	if err != nil {
		return nil, err
	}
	if record != nil {
		if record.Get("status").(string) == "completed" {
			return &OrderJob{
				MerchantOrderID: merchantOrderID,
				State:           "COMPLETED",
			}, nil
		}
		if record.Get("status").(string) == "failed" {
			return &OrderJob{
				MerchantOrderID: merchantOrderID,
				State:           "FAILED",
			}, nil
		}
		//checking if the order is expired
		if time.Now().After(record.Get("expires").(time.Time)) {
			return &OrderJob{
				MerchantOrderID: merchantOrderID,
				State:           "EXPIRED",
			}, nil
		}
	}
	req, _ := http.NewRequest("GET", os.Getenv("PG_API_URL")+"/checkout/v2/order/"+merchantOrderID+"/status", nil)
	fmt.Println("Request URL:", req.URL.String())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "O-Bearer "+token)
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	var osr OrderResponse
	if err := json.NewDecoder(resp.Body).Decode(&osr); err != nil {
		return nil, fmt.Errorf("JSON decode error: %w", err)
	}
	job := &OrderJob{
		MerchantOrderID: osr.OrderID,
		State:           osr.State,
		Expires:         time.Unix(osr.ExpireAt, 0),
		NextPoll:        time.Now().Add(20 * time.Second),
		Interval:        3 * time.Second,
	}
	return job, nil

}

func InitPolling(app *pocketbase.PocketBase, workers int) {
	if workers <= 0 {
		log.Println("Invalid number of workers, defaulting to 5")
		workers = 5
	}
	log.Printf("Initializing polling with %d workers", workers)
	// Channel for job processing
	jobCh := make(chan *OrderJob)
	var wg sync.WaitGroup

	// Workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				resp, err := getOrderStatus(app, job.MerchantOrderID)
				if err != nil {
					log.Printf("poll error %s: %v", job.MerchantOrderID, err)
				} else {
					state := resp.State
					//print out response
					respBytes, _ := json.MarshalIndent(resp, "", "  ")
					log.Println(string(respBytes))
					log.Printf("order %s → %s", job.MerchantOrderID, state)
					if state == "EXPIRED" {
						log.Println(fmt.Sprintf("Expired after no valid response merchantOrderID = '%s'", job.MerchantOrderID))
						continue
					}
					if state == "COMPLETED" || state == "FAILED" {
						// TODO: update DB, notify UI
						log.Printf("order %s completed", job.MerchantOrderID)
						record, err := app.FindFirstRecordByFilter("payments", fmt.Sprintf("merchantOrderID = '%s'", job.MerchantOrderID))
						if err != nil {
							log.Println(err)
							continue
						}
						record.Set("status", state)
						err = app.Save(record)
						if err != nil {
							log.Println(err)
						}

						continue
					}
					// Reschedule next poll
					job.NextPoll = time.Now().Add(job.Interval)
					job.Interval = bumpInterval(job.Interval)
					jobQueue.Lock()
					jobQueue.jobs = append(jobQueue.jobs, job)
					jobQueue.Unlock()
				}
			}
		}()
	}

	// Dispatcher
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer close(jobCh)
		for range ticker.C {
			now := time.Now()
			jobQueue.Lock()
			var pending []*OrderJob
			for _, job := range jobQueue.jobs {
				if now.After(job.Expires) {
					log.Printf("job %s expired", job.MerchantOrderID)
					continue
				}
				if now.After(job.NextPoll) {
					jobCh <- job
				} else {
					pending = append(pending, job)
				}
			}
			jobQueue.jobs = pending
			jobQueue.Unlock()
			// if len(pending) == 0 {
			// 	ticker.Stop()
			// 	return
			// }
		}
	}()

	wg.Wait()
}

func bumpInterval(current time.Duration) time.Duration {
	switch {
	case current < 6*time.Second:
		return 6 * time.Second
	case current < 10*time.Second:
		return 10 * time.Second
	case current < 30*time.Second:
		return 30 * time.Second
	default:
		return time.Minute
	}
}

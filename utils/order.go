package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
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

func AddOrderJob(id string, expireAfterSecs int64) {
	fmt.Println("Adding order job:", id, "with expiration in", expireAfterSecs, "seconds")
	if expireAfterSecs <= 0 {
		return
	}
	now := time.Now()
	job := &OrderJob{
		MerchantOrderID: id,
		Expires:         now.Add(time.Duration(expireAfterSecs) * time.Second),
		NextPoll:        now.Add(20 * time.Second),
		Interval:        3 * time.Second,
	}
	jobQueue.Lock()
	jobQueue.jobs = append(jobQueue.jobs, job)
	jobQueue.Unlock()
}

func getOrderStatus(merchantOrderID string) (*OrderJob, error) {
	token, err := GetPGAuthToken()
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequest("GET", os.Getenv("PG_API_URL")+"/v2/orders/"+merchantOrderID, nil)
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
		Expires:         time.Unix(osr.ExpireAt, 0),
		NextPoll:        time.Now().Add(20 * time.Second),
		Interval:        3 * time.Second,
	}
	return job, nil

}

func InitPolling(workers int) {
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
				resp, err := getOrderStatus(job.MerchantOrderID)
				if err != nil {
					log.Printf("poll error %s: %v", job.MerchantOrderID, err)
				} else {
					state := resp.State
					log.Printf("order %s → %s", job.MerchantOrderID, state)
					if state == "COMPLETED" || state == "FAILED" {
						// TODO: update DB, notify UI
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
			if len(pending) == 0 {
				ticker.Stop()
				return
			}
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

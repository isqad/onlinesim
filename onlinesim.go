package onlinesim

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"
)

const MaxRetries = 3
const OnlinesimApiEndpoint = `https://onlinesim.ru/api`

func init() {
	rand.Seed(time.Now().UnixNano())
}

type BalanceResponse struct {
	Response string
	Balance  string
	Zbalance int32
}

func GetBalance(balance *float64) error {
	apiKey := os.Getenv("ONLINESIM_API_KEY")
	action := `getBalance`

	url := fmt.Sprintf(`%s/%s.php?apikey=%s`, OnlinesimApiEndpoint, action, apiKey)

	return retry(MaxRetries, time.Second, func() error {
		var onlineSimResponse BalanceResponse
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: tr}
		resp, err := client.Get(url)

		if err != nil {
			log.Fatal(err)
			return err
		}

		defer resp.Body.Close()

		body, _ := ioutil.ReadAll(resp.Body)

		log.Print(string(body[:]))

		if err = json.Unmarshal(body, &onlineSimResponse); err != nil {
			log.Fatal(err)
			return nil
		}

		if onlineSimResponse.Response != `1` {
			log.Fatal(onlineSimResponse)
			return nil
		}

		log.Print(onlineSimResponse)

		money, err := strconv.ParseFloat(onlineSimResponse.Balance, 64)
		if err != nil {
			log.Fatal(err)
			return nil
		}

		*balance = money

		return nil
	})
}

func GetNumber(number *string) error {

}

// https://upgear.io/blog/simple-golang-retry-function/
func retry(attempts int, sleep time.Duration, f func() error) error {
	if err := f(); err != nil {
		if s, ok := err.(stop); ok {
			// Return the original error for later checking
			return s.error
		}

		if attempts--; attempts > 0 {
			// Add some randomness to prevent creating a Thundering Herd
			jitter := time.Duration(rand.Int63n(int64(sleep)))
			sleep = sleep + jitter/2

			time.Sleep(sleep)
			return retry(attempts, 2*sleep, f)
		}
		return err
	}

	return nil
}

type stop struct {
	error
}

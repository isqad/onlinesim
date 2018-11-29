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

func apiUrl(action string) string {
	apiKey := os.Getenv("ONLINESIM_API_KEY")
	return fmt.Sprintf(`%s/%s.php?apikey=%s`, OnlinesimApiEndpoint, action, apiKey)
}

func client() *http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	return &http.Client{Transport: tr}
}

func GetBalance(balance *float64) error {
	return retry(MaxRetries, time.Second, func() error {
		var onlineSimResponse BalanceResponse

		resp, err := client().Get(apiUrl(`getBalance`))

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

type GetNumberResponse struct {
	Response int32
	Tzid     int32
}

func GetNumber(service string, id *int32) error {
	return retry(MaxRetries, time.Second, func() error {
		var getNumResponse GetNumberResponse

		url := fmt.Sprintf(`%s&service=%s&country=86`, apiUrl(`getNum`), service)
		log.Print(url)
		resp, err := client().Get(url)

		if err != nil {
			log.Fatal(err)
			return err
		}

		defer resp.Body.Close()

		body, _ := ioutil.ReadAll(resp.Body)

		log.Print(string(body[:]))

		if err = json.Unmarshal(body, &getNumResponse); err != nil {
			log.Fatal(err)
			return nil
		}

		if getNumResponse.Response != 1 {
			log.Fatal(getNumResponse)
			return nil
		}

		log.Print(getNumResponse)

		*id = getNumResponse.Tzid

		return nil
	})
}

type StateResponse struct {
	Response      string
	Tzid          string
	Service       string
	Number        string
	Msg           string
	Time          string
	Form          string
	ForwardStatus string
	ForwardNumber string
	Country       string
}

func GetState(id int32, messageToCode int32, state *StateResponse) error {
	return retry(MaxRetries, time.Second, func() error {
		var stateResponse StateResponse
		url := fmt.Sprintf(`%s&tzid=%d&message_to_code=%d`,
			apiUrl(`getState`), id, messageToCode)

		log.Print(url)

		resp, err := client().Get(url)

		if err != nil {
			log.Fatal(err)
			return err
		}

		defer resp.Body.Close()

		body, _ := ioutil.ReadAll(resp.Body)

		log.Print(string(body[:]))

		if err = json.Unmarshal(body, &stateResponse); err != nil {
			log.Fatal(err)
			return nil
		}

		if stateResponse.Response != `1` {
			log.Fatal(stateResponse)
			return nil
		}

		log.Print(stateResponse)

		*state = stateResponse

		return nil
	})

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

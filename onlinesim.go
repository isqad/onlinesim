package onlinesim

import (
	"crypto/tls"
	"encoding/json"
	"errors"
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

type JsonResponse int

// Разновидности ответов
const (
	ResponseSuccess       JsonResponse = iota
	ResponseFail                       // Ошибка
	ResponseSmsWait                    // Ожидается ответ
	ResponseNoNums                     // Нет подходящих номеров
	ResponseTzInpool                   // Операция ожидает выделения номера
	ResponseTzOverEmpty                // Ответ не поступил за отведенное время
	ResponseTzNumAnswer                // Поступил ответ
	ResponseTzOverOk                   // Операция завершена
	ResponseErrorNoTzid                // Не указан tzid
	ResponseErrorNoOps                 // Нет операций
	ResponseAccountIdFail              // Необходимо пройти идентификацию для заказа номера
)

func (a *JsonResponse) UnmarshalJSON(b []byte) error {
	if b[0] == '1' {
		*a = ResponseSuccess
		return nil
	}

	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	switch s {
	case "1":
		*a = ResponseSuccess
	case "TZ_NUM_WAIT":
		*a = ResponseSmsWait
	case "WARNING_NO_NUMS":
		*a = ResponseNoNums
	case "TZ_INPOOL":
		*a = ResponseTzInpool
	case "TZ_OVER_EMPTY":
		*a = ResponseTzOverEmpty
	case "TZ_NUM_ANSWER":
		*a = ResponseTzNumAnswer
	case "TZ_OVER_OK":
		*a = ResponseTzOverOk
	case "ERROR_NO_TZID":
		*a = ResponseErrorNoTzid
	case "ERROR_NO_OPERATIONS":
		*a = ResponseErrorNoOps
	case "ACCOUNT_IDENTIFICATION_REQUIRED":
		*a = ResponseAccountIdFail
	default:
		*a = ResponseFail
	}

	return nil
}

type NumberResponse struct {
	Response JsonResponse
	Tzid     int32
}

type Msg struct {
	Service string
	Msg     string
}

type StateResponse struct {
	Response      JsonResponse
	Tzid          int32
	Service       string
	Number        string
	Msg           []Msg
	Time          int32
	Form          string
	ForwardStatus string
	ForwardNumber string
	Country       int32
}

type BalanceResponse struct {
	Response JsonResponse
	Balance  string
	Zbalance string
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Public: получение текущего баланса
func GetBalance(balance *float64) error {
	return retry(MaxRetries, time.Second, func() error {
		var onlineSimResponse BalanceResponse

		resp, err := client().Get(apiUrl(`getBalance`))

		if err != nil {
			log.Print(err)
			return errors.New("Request failed")
		}

		defer resp.Body.Close()

		body, _ := ioutil.ReadAll(resp.Body)

		log.Print(string(body[:]))

		if err = json.Unmarshal(body, &onlineSimResponse); err != nil {
			log.Print(err)

			return stop{error: err}
		}

		log.Print(onlineSimResponse)

		if onlineSimResponse.Response == ResponseFail {
			return stop{error: errors.New("Response Fail")}
		}

		money, err := strconv.ParseFloat(onlineSimResponse.Balance, 64)
		if err != nil {
			log.Print(err)
			return stop{error: err}
		}

		*balance = money

		return nil
	})
}

func GetNumber(service string) ([]string, int32, error) {
	var idOp int32

	states := make([]StateResponse, 2, 2)
	numbers := make([]string, 2, 2)

	// запросили номер
	// получили ид операции
	if err := numberReq(service, &idOp); err != nil {
		return []string{}, 0, err
	}

	if err := getState(idOp, 1, &states); err != nil {
		return []string{}, 0, err
	}

	for i, s := range states {
		numbers[i] = s.Number
	}

	return numbers, idOp, nil
}

func GetSms(idOp int32, smsText *[]string, tries int) error {
	return retry(tries, time.Second, func() error {
		states := make([]StateResponse, 2, 2)
		texts := make([]string, 2, 2)

		getState(idOp, 1, &states)
		log.Print(states)
		log.Print(states)

		for _, s := range states {
			if s.Response == ResponseTzNumAnswer {
				for j, m := range s.Msg {
					texts[j] = m.Msg
				}
			}

			if s.Response == ResponseSmsWait {
				return errors.New("Waiting response. Retry...")
			}

			if s.Response != ResponseTzNumAnswer {
				log.Print(s)
				return stop{error: errors.New("No valid response")}
			}
		}

		*smsText = texts

		return nil
	})
}

// Private

type stop struct {
	error
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

func getState(id int32, messageToCode int32, states *[]StateResponse) error {
	return retry(MaxRetries, time.Second, func() error {
		stateResponse := make([]StateResponse, 2, 2)

		url := fmt.Sprintf(`%s&tzid=%d&message_to_code=%d&msg_list=1`,
			apiUrl(`getState`), id, messageToCode)

		log.Print(url)

		resp, err := client().Get(url)

		if err != nil {
			log.Print(err)
			return err
		}

		defer resp.Body.Close()

		body, _ := ioutil.ReadAll(resp.Body)

		log.Print(string(body[:]))

		if err = json.Unmarshal(body, &stateResponse); err != nil {
			log.Print(err)
			return stop{error: err}
		}

		if stateResponse[0].Response == ResponseFail {
			log.Print(stateResponse)
			return stop{error: errors.New("Response Fail")}
		}

		*states = stateResponse

		return nil
	})
}

func numberReq(service string, idOp *int32) error {
	return retry(MaxRetries, time.Second, func() error {
		var numResponse NumberResponse

		// Запрос на номер
		url := fmt.Sprintf(`%s&service=%s&country=7`, apiUrl(`getNum`), service)
		log.Print(url)
		resp, err := client().Get(url)

		if err != nil {
			log.Print(err)
			return err
		}

		defer resp.Body.Close()

		body, _ := ioutil.ReadAll(resp.Body)

		log.Print(string(body[:]))

		if err = json.Unmarshal(body, &numResponse); err != nil {
			log.Print(err)

			return stop{error: err}
		}

		if numResponse.Response == ResponseFail {
			log.Print(numResponse)
			return stop{error: errors.New("Response Fail")}
		}

		*idOp = numResponse.Tzid

		return nil
	})
}

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type PdResponse interface {
	GetStatus() string
	GetMessage() string
	IsSuccess() bool
	ShouldRetry() bool
}

type PdAcceptedResponse struct {
	Status   string `json:"status"`
	Message  string `json:"message"`
	DedupKey string `json:"dedup_key"`
}

func (r PdAcceptedResponse) GetStatus() string {
	return r.Status
}

func (r PdAcceptedResponse) GetMessage() string {
	return r.Message
}

func (r PdAcceptedResponse) IsSuccess() bool {
	return true
}

func (r PdAcceptedResponse) ShouldRetry() bool {
	return true
}

type PdErrorResponse struct {
	Status  string   `json:"status"`
	Message string   `json:"message"`
	Errors  []string `json:"errors"`
}

func (r PdErrorResponse) GetStatus() string {
	return r.Status
}

func (r PdErrorResponse) GetMessage() string {
	return r.Message
}

func (r PdErrorResponse) IsSuccess() bool {
	return false
}

func (r PdErrorResponse) ShouldRetry() bool {
	return false
}

type PdRateResponse struct{}

func (r PdRateResponse) GetStatus() string {
	return "Too many requests"
}

func (r PdRateResponse) GetMessage() string {
	return "Too many requests"
}

func (r PdRateResponse) IsSuccess() bool {
	return false
}

func (r PdRateResponse) ShouldRetry() bool {
	return true
}

type PagerDutyCommunicator struct {
	ApiKey    string
	ServiceId string
}

func TimeToFailureAsString(estimatedTimeToFail float64) string {
	hours := int64(estimatedTimeToFail / 3600)
	mins := int64((estimatedTimeToFail - float64(hours*3600)) / 60)
	seconds := int64(estimatedTimeToFail - float64(hours*3600) - float64(mins*60))

	return fmt.Sprintf("%d hrs, %d mins and %d seconds", hours, mins, seconds)
}

/**
builds a request body json with the provided parameters
returns either a Reader containing the content or an error if it could not be marshalled
*/
func (comm *PagerDutyCommunicator) MakeBody(alertSummary string, timestamp time.Time, componentName string, currentValue float64, failureThreshold float64, estimatedTimeToFailure float64) (io.Reader, error) {
	bodyContent := map[string]interface{}{
		"payload": map[string]interface{}{
			"summary":   alertSummary,
			"timestamp": timestamp.Format(time.RFC3339),
			"source":    "connary",
			"component": componentName,
			"custom_details": map[string]interface{}{
				"currentValue":           currentValue,
				"failureThreshold":       failureThreshold,
				"estimatedTimeToFailure": TimeToFailureAsString(estimatedTimeToFailure),
			},
		},
		"event_action": "trigger",
		"client":       "connary",
	}

	byteBuffer, marshalErr := json.Marshal(&bodyContent)
	if marshalErr != nil {
		return nil, marshalErr
	}

	return bytes.NewReader(byteBuffer), nil
}

func (comm *PagerDutyCommunicator) SendRequest(reqbody io.Reader) (PdResponse, error) {
	req, reqErr := http.NewRequest("POST", "https://events.pagerduty.com/v2/enqueue", reqbody)
	if reqErr != nil {
		log.Print("PagerDutyCommunicator.SendRequest ERROR could not create request: ", reqErr)
		return nil, reqErr
	}

	req.Header.Add("Authorization", fmt.Sprintf("Token token=%s", comm.ApiKey))
	req.Header.Add("Accept", "application/json")
	client := &http.Client{
		Timeout: 30,
	}

	resp, sendErr := client.Do(req)
	if sendErr != nil {
		log.Print("PagerDutyCommunicator.SendRequest ERROR could not send: ", sendErr)
		return nil, sendErr
	}

	bodyContent, readErr := ioutil.ReadAll(resp.Body)
	if readErr != nil {
		log.Print("PagerDutyCommunicator.SendRequest ERROR could not read response: ", readErr)
		return nil, readErr
	}

	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		var response PdAcceptedResponse
		unmarshalErr := json.Unmarshal(bodyContent, &response)
		if unmarshalErr != nil {
			log.Print("PagerDutyCommunicator.SendRequest ERROR could not understand response: ", unmarshalErr)
			return nil, unmarshalErr
		}
		return response, nil
	} else if resp.StatusCode == 429 {
		response := PdRateResponse{}
		return response, nil
	} else {
		var response PdErrorResponse
		unmarshalErr := json.Unmarshal(bodyContent, &response)
		if unmarshalErr != nil {
			log.Print("PagerDutyCommunicator.SendRequest ERROR could not understand response: ", unmarshalErr)
			return nil, unmarshalErr
		}
		return response, nil
	}
}

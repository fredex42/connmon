package main

import (
	"encoding/json"
	"io/ioutil"
	"testing"
	"time"
)

func TestPagerDutyCommunicator_MakeBody(t *testing.T) {
	comm := &PagerDutyCommunicator{
		ApiKey:    "some-key",
		ServiceId: "some-service",
	}

	currentTime := time.Now()
	bodyReader, err := comm.MakeBody("Your desk is on fire", currentTime, "tabletop", 60, 100, 30)
	if err != nil {
		t.Error("MakeBody failed unexpectely: ", err)
	}

	content, _ := ioutil.ReadAll(bodyReader)
	var actualData map[string]interface{}
	unmarshalErr := json.Unmarshal(content, &actualData)
	if unmarshalErr != nil {
		t.Error("could not unmarshal data: ", unmarshalErr)
	}

	payload := actualData["payload"].(map[string]interface{})
	if payload["summary"].(string) != "Your desk is on fire" {
		t.Error("summary was incorrect")
	}
	if payload["timestamp"].(string) != currentTime.Format(time.RFC3339) {
		t.Error("timestamp was incorrect")
	}
	if payload["component"].(string) != "tabletop" {
		t.Error("component was incorrect")
	}
	customDetails := payload["custom_details"].(map[string]interface{})
	if customDetails["currentValue"].(float64) != 60 {
		t.Error("currentValue was incorrect")
	}
	if customDetails["failureThreshold"].(float64) != 100 {
		t.Error("failureThreshold was incorrect")
	}
	if customDetails["estimatedTimeToFailure"].(string) != "0 hrs, 0 mins and 30 seconds" {
		t.Error("estimatedTimeToFailure was incorrect, got ", customDetails["estimatedTimeToFailure"].(string))
	}
}

package main

import (
	"testing"
	"time"
)

func TestEstimateTimeToFailure(t *testing.T) {
	startTime, _ := time.Parse(time.RFC3339, "2020-01-02T03:04:05Z")
	testRecords := []Record{
		{
			timestamp: startTime.Add(40 * time.Second),
			value:     25,
		},
		{
			timestamp: startTime.Add(30 * time.Second),
			value:     20,
		},
		{
			timestamp: startTime.Add(20 * time.Second),
			value:     15,
		},
		{
			timestamp: startTime.Add(10 * time.Second),
			value:     10,
		},
		{
			timestamp: startTime.Add(0),
			value:     5,
		},
	}

	estimatedFailureSeconds := EstimateTimeToFailure(&testRecords, 50)

	if estimatedFailureSeconds != 50 {
		//in this example, we are putting on 5 units every 10 seconds, i.e. 1/2 unit per second
		//"now" is the most recent value, i.e. we are at 25 units with a failure threshold of 50 units
		//it has taken us 40 seconds to put on 20 units from 5 to 25, therefore to put on another 25 units is
		//40 seconds + 10 seconds = 50 seconds.
		t.Errorf("expected failure in 50 seconds, got %f", estimatedFailureSeconds)
	}
}

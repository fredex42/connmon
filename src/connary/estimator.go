package main

import (
	"github.com/davecgh/go-spew/spew"
	"log"
)

func sum(values []float64) float64 {
	var total float64 = 0
	for _, v := range values {
		total += v
	}
	return total
}

func EstimateTimeToFailure(records *[]Record, failureThreshold float64) float64 {
	//first, get the rate of change between each record
	changeRates := make([]float64, len(*records)-1)
	for i := 0; i < len(*records)-1; i++ {
		rec := (*records)[i]
		nextRec := (*records)[i+1]
		timediff := float64(nextRec.timestamp.UnixNano()-rec.timestamp.UnixNano()) / 1e9

		changeRates[i] = (nextRec.value - rec.value) / timediff
	}

	log.Printf("DEBUG EstimateTimeToFailure rate set is %s", spew.Sdump(changeRates))
	//now, average up the changerates
	averageRate := sum(changeRates) / float64(len(changeRates))
	log.Printf("DEBUG EstimateTimeToFailure averageRate is %f counts per second based on a sample of %d", averageRate, len(*records))

	//ok, with an average rate of change we can do a simple linear estimate of time to failure
	mostRecentValue := (*records)[0].value
	countsBeforeFailure := failureThreshold - mostRecentValue
	timeToFailure := countsBeforeFailure / averageRate //counts / (counts / second) => second
	return timeToFailure
}

package main

import "log"

func SendPdAlert(matchField string, matchValue string, count float64, estimatedTimeToFail float64) {
	hours := int64(estimatedTimeToFail / 3600)
	mins := int64((estimatedTimeToFail - float64(hours*3600)) / 60)
	seconds := int64(estimatedTimeToFail - float64(hours*3600) - float64(mins*60))

	log.Printf("ALERT! %s %s is over threshold at %f. Estimated failure time is %d hrs, %d mins and %d seconds ", matchField, matchValue, count, hours, mins, seconds)
}

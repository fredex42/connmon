package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	elasticsearch6 "github.com/elastic/go-elasticsearch/v6"
	"io/ioutil"
	"log"
	"os"
	"time"
)

type Record struct {
	timestamp time.Time
	value     float64 //all ES numbers come back as float
}

func RecordFromMap(incomingData map[string]interface{}, timestampFieldName string, valueFieldName string) (Record, error) {
	docSource, haveDocSource := incomingData["_source"].(map[string]interface{})
	if !haveDocSource {
		log.Printf("RecordFromMap DEBUG offending data was %s", spew.Sdump(incomingData))
		return Record{}, errors.New("record had no source document")
	}

	tsString, haveTsValue := docSource[timestampFieldName]
	if !haveTsValue {
		log.Printf("RecordFromMap DEBUG offending data was %s", spew.Sdump(docSource))
		return Record{}, errors.New("record had no timestamp field")
	}
	tsValue, parseErr := time.Parse(time.RFC3339Nano, tsString.(string))
	if parseErr != nil {
		return Record{}, parseErr
	}
	valueIf, haveValue := docSource[valueFieldName]
	if !haveValue {
		log.Printf("RecordFromMap DEBUG offending data was %s", spew.Sdump(docSource))
		return Record{}, errors.New("record had no count field")
	}
	actualValue, canConvert := valueIf.(float64)
	if !canConvert {
		log.Printf("RecordFromMap DEBUG offending data was %s", spew.Sdump(docSource))
		return Record{}, errors.New(fmt.Sprintf("could not convert %s from %s to float", spew.Sdump(valueIf), spew.Sdump(incomingData)))
	}
	return Record{
		timestamp: tsValue,
		value:     actualValue,
	}, nil
}

func MakeSearchRequest(fieldNamePtr *string, fieldValuePtr *string) []byte {
	searchParamName := fmt.Sprintf("%s.keyword", *fieldNamePtr)
	mapData := map[string]interface{}{
		"query": map[string]interface{}{
			"match": map[string]string{
				searchParamName: *fieldValuePtr,
			},
		},
		"sort": map[string]interface{}{
			"timestamp": map[string]string{
				"order": "desc",
			},
		},
	}

	marshalledData, marshalErr := json.Marshal(&mapData)
	if marshalErr != nil {
		panic(fmt.Sprintf("could not marshal request, this indicates a code bug: %s", marshalErr))
	}
	return marshalledData
}

func RequestRecords(indexNamePtr *string, matchFieldPtr *string, matchFieldValuePtr *string,
	timestampFieldPtr *string, countFieldPtr *string, sampleLengthPtr *int, ctx context.Context, esConn *elasticsearch6.Client) ([]Record, error) {
	requestBody := bytes.NewReader(MakeSearchRequest(matchFieldPtr, matchFieldValuePtr))
	res, err := esConn.Search(
		esConn.Search.WithContext(ctx),
		esConn.Search.WithBody(requestBody),
		esConn.Search.WithIndex(*indexNamePtr),
		esConn.Search.WithSize(*sampleLengthPtr),
	)
	if err != nil {
		log.Printf("RequestRecords ERROR: elasticsearch failed: %s", err)
		return nil, err
	}

	returnedContent, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		log.Printf("RequestRecords ERROR: could not read server response: %s", readErr)
		return nil, readErr
	}

	var contentMap map[string]interface{}
	unMarshalErr := json.Unmarshal(returnedContent, &contentMap)
	if unMarshalErr != nil {
		log.Printf("RequestRecords ERROR: could not understand server response: %s", unMarshalErr)
		return nil, unMarshalErr
	}

	hitsArray := contentMap["hits"].(map[string]interface{})["hits"].([]interface{})
	recordsToReturn := make([]Record, len(hitsArray))

	for i, hit := range hitsArray {
		rec, err := RecordFromMap(hit.(map[string]interface{}), *timestampFieldPtr, *countFieldPtr)
		if err != nil {
			log.Printf("RequestRecords ERROR: could not convert record %d from map: %s", i, err)
			return nil, err
		}
		recordsToReturn[i] = rec
	}
	return recordsToReturn, nil
}

/**
returns True if the first record in the list has a count over `threshold`.
this assumes that the first record in the list is the most recent, which should be the case due to the sort
requested of ElasticSearch
*/
func IsOverThreshold(recordsList *[]Record, threshold float64) bool {
	return (*recordsList)[0].value > threshold
}

func SendAlertWithRetry(matchFieldPtr *string, matchFieldValuePtr *string, failureThresholdPtr *float64, estimatedFailureSeconds float64, recordList *[]Record, pdCommunicator *PagerDutyCommunicator) {
	alertSummary := fmt.Sprintf("%s %s is out of range at %f", *matchFieldPtr, *matchFieldValuePtr, (*recordList)[0].value)
	requestBody, makeBodyErr := pdCommunicator.MakeBody(alertSummary, time.Now(), *matchFieldValuePtr, (*recordList)[0].value, *failureThresholdPtr, estimatedFailureSeconds)
	if makeBodyErr == nil {
		response, err := pdCommunicator.SendRequest(requestBody)
		if err != nil {
			log.Panic("ERROR: could not send request: ", err)
		} else {
			if !response.IsSuccess() && response.ShouldRetry() {
				log.Printf("PD is rejecting as too many requests, retrying in 5 seconds")
				time.Sleep(5 * time.Second)
				SendAlertWithRetry(matchFieldPtr, matchFieldValuePtr, failureThresholdPtr, estimatedFailureSeconds, recordList, pdCommunicator)
			} else if !response.IsSuccess() {
				log.Printf("ERROR PD rejected notification: %s %s", response.GetStatus(), response.GetMessage())
			} else {
				successResponse := response.(PdAcceptedResponse)
				log.Printf("Registered message, dedup key is %s", successResponse.DedupKey)
			}
		}
	} else {
		log.Panic("could not make body for PD alert: ", makeBodyErr)
	}
}

func main() {
	esHostPtr := flag.String("es-host", "http://localhost:9200", "URI of ElasticSearch host to connect to")
	indexNamePtr := flag.String("index-name", "db_connection_metrix", "Elasticsearch index to use")
	matchFieldPtr := flag.String("match-field", "", "Only check records with this field name matching the given value")
	matchFieldValuePtr := flag.String("match-value", "", "Value that must be present in the match-field field")
	timestampFieldPtr := flag.String("timestamp-field", "timestamp", "Field that contains a recognised timestamp for the record")
	countFieldPtr := flag.String("count-field", "count", "Field that contains the data to check. Must be a numeric type.")
	countFieldThresholdPtr := flag.Float64("threshold", 1, "Alert if the value of count-field is higher than this number")
	failureThresholdPtr := flag.Float64("failure-threshold", 200, "Assume failure if the count gets this high. Used to estimate time-to-failure")
	sampleLengthPtr := flag.Int("sample-length", 100, "When performing time-to-failure estimate, sample this many records")
	//continueousDelayPtr := flag.Int("continuous-delay", 0, "If set, then run continously; checking at an interval of this many seconds")
	flag.Parse()

	pdKey := os.Getenv("PAGERDUTY_KEY")
	if pdKey == "" {
		log.Print("You must specify a valid pagerduty key in the PAGERDUTY_KEY environment variable")
		os.Exit(1)
	}

	pdService := os.Getenv("PAGERDUTY_SERVICE")
	if pdService == "" {
		log.Print("You must specify a valid pagerduty service id in the PAGERDUTY_SERVICE environment variable")
		os.Exit(1)
	}

	pdCommunicator := &PagerDutyCommunicator{
		ApiKey:    pdKey,
		ServiceId: pdService,
	}

	ctx := context.Background()

	cfg := elasticsearch6.Config{
		Addresses: []string{
			*esHostPtr,
		},
	}

	esConn, err := elasticsearch6.NewClient(cfg)
	if err != nil {
		log.Fatal(err)
	}

	_, connErr := esConn.Info()
	if connErr != nil {
		log.Fatal(connErr)
	}

	//log.Println(res)
	//log.Println(data)

	recordList, fetchErr := RequestRecords(indexNamePtr, matchFieldPtr, matchFieldValuePtr, timestampFieldPtr, countFieldPtr, sampleLengthPtr, ctx, esConn)
	if fetchErr != nil {
		log.Printf("main ERROR: could not fetch records")
		os.Exit(1)
	}
	spew.Dump(recordList)

	if IsOverThreshold(&recordList, *countFieldThresholdPtr) {
		estimatedFailureSeconds := EstimateTimeToFailure(&recordList, *failureThresholdPtr)
		SendAlertWithRetry(matchFieldPtr, matchFieldValuePtr, failureThresholdPtr, estimatedFailureSeconds, &recordList, pdCommunicator)
		//SendPdAlert(*matchFieldPtr, *matchFieldValuePtr, recordList[0].value, estimatedFailureSeconds)
	} else {
		log.Printf("%s %s is currently in-range at %f", *matchFieldPtr, *matchFieldValuePtr, recordList[0].value)
	}
}

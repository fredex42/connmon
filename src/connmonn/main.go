package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	elasticsearch6 "github.com/elastic/go-elasticsearch/v6"
	"github.com/elastic/go-elasticsearch/v6/esapi"
	_ "github.com/lib/pq" //import is needed as the postgres driver for database/sql
	"log"
	"strings"
	"time"
)

type DataEntry struct {
	DBName    string    `json:"datname"`
	Count     int       `json:"count"`
	Host      string    `json:"host"`
	Timestamp time.Time `json:"timestamp"`
}

func main() {
	dbHostPtr := flag.String("db-host", "localhost", "Database host to connect to")
	dbUserPtr := flag.String("db-user", "postgres", "Username for the database")
	dbPwdPtr := flag.String("db-pass", "", "Password for the database")
	dbPortPtr := flag.String("db-port", "5432", "TCP Port to connect to the database")
	dbSSLMode := flag.String("db-ssl-mode", "disable", "SSL mode for database. Consult https://godoc.org/github.com/lib/pq for options")

	esHostPtr := flag.String("es-host", "http://elasticsearch:9200", "URI of ElasticSearch host to connect to")
	indexNamePtr := flag.String("index-name", "db-connections", "Elasticsearch index to use")
	flag.Parse()

	connStr := fmt.Sprintf("user=%s password=%s host=%s port=%s sslmode=%s dbname=postgres", *dbUserPtr, *dbPwdPtr, *dbHostPtr, *dbPortPtr, *dbSSLMode)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	rows, err := db.Query("select distinct(datname),count(datid) from pg_stat_activity group by datname")
	if err != nil {
		log.Fatal(err)
	}

	var data []string

	for rows.Next() {
		var entry DataEntry

		err := rows.Scan(&entry.DBName, &entry.Count)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Got %d connections on %s\n", entry.Count, entry.DBName)

		entry.Host = *dbHostPtr
		entry.Timestamp = time.Now()
		log.Println(entry)
		content, err := json.Marshal(&entry)
		if err != nil {
			log.Fatal(err)
		}
		data = append(data, string(content))
	}

	cfg := elasticsearch6.Config{
		Addresses: []string{
			*esHostPtr,
		},
	}

	esConn, err := elasticsearch6.NewClient(cfg)
	if err != nil {
		log.Fatal(err)
	}

	res, err := esConn.Info()
	if err != nil {
		log.Fatal(err)
	}

	log.Println(res)
	log.Println(data)

	for i := 0; i < len(data); i++ {
		req := esapi.IndexRequest{
			Index:   *indexNamePtr,
			Body:    strings.NewReader(data[i]),
			Refresh: "true",
		}

		res, _ := req.Do(context.Background(), esConn)
		defer res.Body.Close()
		if res.IsError() {
			log.Printf("Could not index result: %s", res.Status())
		} else {
			log.Printf("Indexed doc %d", i)
		}
	}

	log.Printf("completed")
}

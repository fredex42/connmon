#!/bin/sh

if [ "$SLEEP_TIME" == "" ]; then
  SLEEP_TIME=120
fi

while `true`; do
  /usr/local/bin/connmonn --db-host $DB_HOST --db-pass $DB_PASS --db-user $DB_USER --es-host $ES_URI --index-name $INDEX_NAME
  if [ "$?" != "0" ]; then
    echo Command exited with an error
    exit $?
  fi
  sleep $SLEEP_TIME
done

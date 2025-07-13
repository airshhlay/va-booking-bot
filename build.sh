#!/bin/bash

APP_NAME=vabooker
LOG_FILE=log.txt

go mod tidy
go build -o "$APP_NAME" .
if [ $? -ne 0 ]; then
  echo ">> Build failed."
  exit 1
fi

nohup ./"$APP_NAME" > "$LOG_FILE" 2>&1 &
echo ">> App started in background. PID: $!"
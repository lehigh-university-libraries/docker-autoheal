#!/usr/bin/env bash

set -euo pipefail

WEBHOOK_URL=http://localhost:8080

docker compose -f ./ci/docker-compose.yml up -d

echo "Waiting for webhook endpoint to come online"
until curl -s -o /dev/null -w "%{http_code}" "$WEBHOOK_URL" | grep -q "200"; do
  sleep 1
done

echo "Starting our docker healthy monitor in the background"
nohup ./docker-autoheal --interval "5s" --webhook-url $WEBHOOK_URL --webhook-key foo &
PID=$!

echo "Stopping our test container"
docker stop ci-foo-1 > /dev/null 2>&1

# wait for monitor to detect/fix
sleep 10

docker logs ci-webhook-1 2>&1 | grep -q "Unhealthy services"
echo "webhook received the failure payload"

sleep 10

# make sure the webhook received the recovered payload
docker logs ci-webhook-1 2>&1 | grep -q "All is well"
echo "webhook received the recovered payload"

echo "Cleanup docker compose"
docker compose -f ./ci/docker-compose.yml down

echo "Stop the monitor service"
kill $PID

echo "Test completed successfully."

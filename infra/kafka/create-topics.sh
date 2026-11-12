#!/usr/bin/env bash
# Membuat topik Kafka yang dipakai Saybu. Dijalankan sekali oleh container kafka-init.
set -euo pipefail

BOOTSTRAP="${KAFKA_BOOTSTRAP:-kafka:9092}"
KAFKA_BIN=/opt/kafka/bin

topics=(
  order.events.v1
  payment.events.v1
  catalog.events.v1
  notification.commands.v1
)

for topic in "${topics[@]}"; do
  for t in "$topic" "$topic.dlq"; do
    echo "creating topic: $t"
    "$KAFKA_BIN/kafka-topics.sh" \
      --bootstrap-server "$BOOTSTRAP" \
      --create --if-not-exists \
      --topic "$t" \
      --partitions 3 \
      --replication-factor 1
  done
done

echo "topics:"
"$KAFKA_BIN/kafka-topics.sh" --bootstrap-server "$BOOTSTRAP" --list

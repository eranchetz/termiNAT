#!/bin/bash
# Check if Flow Logs are actually capturing data
# Usage: ./scripts/check-flowlogs-data.sh <log-group-name>

set -e

LOG_GROUP="${1}"

if [ -z "$LOG_GROUP" ]; then
    echo "Usage: $0 <log-group-name>"
    echo "Example: $0 /aws/vpc/flowlogs/terminator-1234567890"
    exit 1
fi

echo "Checking Flow Logs data in: $LOG_GROUP"
echo ""

# Check if log group exists
echo "1. Checking if log group exists..."
aws logs describe-log-groups --log-group-name-prefix "$LOG_GROUP" --query 'logGroups[0].[logGroupName,storedBytes,creationTime]' --output table

# Check log streams
echo ""
echo "2. Checking log streams..."
aws logs describe-log-streams --log-group-name "$LOG_GROUP" --order-by LastEventTime --descending --max-items 5 --query 'logStreams[].[logStreamName,lastEventTime,storedBytes]' --output table

# Query for raw records (last 30 minutes)
echo ""
echo "3. Querying for raw Flow Logs records (last 30 minutes)..."
START_TIME=$(date -u -d '30 minutes ago' +%s)
END_TIME=$(date -u +%s)

QUERY_ID=$(aws logs start-query \
    --log-group-name "$LOG_GROUP" \
    --start-time $START_TIME \
    --end-time $END_TIME \
    --query-string 'fields @timestamp, pkt_srcaddr, pkt_dstaddr, bytes, packets, action | limit 20' \
    --query 'queryId' \
    --output text)

echo "Query ID: $QUERY_ID"
echo "Waiting for query to complete..."
sleep 5

# Get results
aws logs get-query-results --query-id "$QUERY_ID" --output json | jq -r '
    if .status == "Complete" then
        if (.results | length) == 0 then
            "❌ No records found in the last 30 minutes"
        else
            "✅ Found \(.results | length) records:",
            "",
            (.results[] | 
                [.[] | select(.field == "@timestamp").value,
                 .[] | select(.field == "pkt_srcaddr").value,
                 .[] | select(.field == "pkt_dstaddr").value,
                 .[] | select(.field == "bytes").value,
                 .[] | select(.field == "action").value] | 
                @tsv)
        end
    else
        "Query status: \(.status)"
    end
'

# Query for aggregated stats
echo ""
echo "4. Querying for aggregated traffic stats..."
QUERY_ID=$(aws logs start-query \
    --log-group-name "$LOG_GROUP" \
    --start-time $START_TIME \
    --end-time $END_TIME \
    --query-string 'fields @timestamp, pkt_dstaddr, bytes | filter action = "ACCEPT" | stats sum(bytes) as total_bytes, count(*) as records by pkt_dstaddr | sort total_bytes desc | limit 10' \
    --query 'queryId' \
    --output text)

echo "Query ID: $QUERY_ID"
echo "Waiting for query to complete..."
sleep 5

aws logs get-query-results --query-id "$QUERY_ID" --output json | jq -r '
    if .status == "Complete" then
        if (.results | length) == 0 then
            "❌ No aggregated data found"
        else
            "✅ Top destinations by bytes:",
            "",
            "Destination IP | Total Bytes | Records",
            "---------------|-------------|--------",
            (.results[] | 
                [.[] | select(.field == "pkt_dstaddr").value,
                 .[] | select(.field == "total_bytes").value,
                 .[] | select(.field == "records").value] | 
                @tsv)
        end
    else
        "Query status: \(.status)"
    end
'

echo ""
echo "Done! If you see no records, possible causes:"
echo "  1. No traffic flowing through NAT Gateway during collection"
echo "  2. Flow Logs not delivering data yet (can take 5-10 minutes)"
echo "  3. All traffic is to private IPs (filtered out by query)"

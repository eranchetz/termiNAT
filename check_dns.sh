#!/bin/bash
end_time=$(($(date +%s) + 1200))
while [ $(date +%s) -lt $end_time ]; do
  echo "=== $(date '+%H:%M:%S') ==="
  echo "A records for terminat.xyz:"
  dig +short terminat.xyz A
  echo -e "\nCNAME for www.terminat.xyz:"
  dig +short www.terminat.xyz CNAME
  echo -e "\nHTTP check:"
  curl -sI http://terminat.xyz | head -1
  echo -e "\n---"
  sleep 60
done

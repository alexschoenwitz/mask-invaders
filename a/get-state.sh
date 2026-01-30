#!/bin/bash

TOKEN_1=$(./register.sh | jq -r '.token')
TOKEN_2=$(./register.sh | jq -r '.token')

echo "$TOKEN_1"
echo "$TOKEN_2"

curl -sX POST localhost:8080/v1/start -H "Authorization: $TOKEN_1"
curl -sX GET localhost:8080/v1/state -H "Authorization: $TOKEN_1" | jq

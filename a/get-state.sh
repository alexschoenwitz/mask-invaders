#!/bin/bash

set -euo pipefail

RESP_1=$(./register.sh)
TOKEN_1=$(echo "$RESP_1" | jq -r '.token')
ID_1=$(echo "$RESP_1" | jq -r '.id')

RESP_2=$(./register.sh)
TOKEN_2=$(echo "$RESP_2" | jq -r '.token')
ID_2=$(echo "$RESP_2" | jq -r '.id')

curl -X POST localhost:8080/v1/start -H "Authorization: $TOKEN_1"

action() {
    token="$1"
    state="$2"

    declare -a cities
    cities=($(echo "$state" | jq -r '.state.cities | keys[]'))
    city1=${cities[0]}
    city2=${cities[1]}

    echo "$city1"
    echo "$ID_1"
    if [[ "$city2" == *"$ID_2" ]]; then
        c="$city1"
        city2="$city1"
        city1="$c"
    fi

    curl -X POST localhost:8080/v1/action -H "Authorization: $token" -d @- <<EOF
    {
        "action": {
            "attack": {
                "from": "$city1",
                "to": "$city2",
                "troops": {
                    "A": 1
                }
            }
        }
    }
EOF
}

for i in $(seq 1 100000); do
    echo "$i"

    STATE=$(curl -X GET localhost:8080/v1/state -H "Authorization: $TOKEN_1")
    echo "$STATE" | jq

    action "$TOKEN_1" "$STATE"
    action "$TOKEN_2" "$STATE"

done

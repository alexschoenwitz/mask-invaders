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
    id=$ID_1

    if [[ "$city2" == *"$ID_2" ]]; then
        c="$city2"
        city2="$city1"
        city1="$c"
        id=$ID_2

    fi

    echo "$token"
    echo "$city1"
    echo "$city2"

    curl -X POST localhost:8080/v1/action -H "Authorization: $token" -d @- <<EOF
    {
        "action": {
            "player": "$id",
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

    sleep 1
}

for i in $(seq 1 100000); do
    echo "$i"

    STATE=$(curl -X GET localhost:8080/v1/state -H "Authorization: $TOKEN_1")
    echo "$STATE" | jq

    action "$TOKEN_1" "$STATE"
    action "$TOKEN_2" "$STATE"

done

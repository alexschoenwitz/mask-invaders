#!/bin/bash

set -euo pipefail

TOKEN_1=$(./register.sh | jq -r '.token')
TOKEN_2=$(./register.sh | jq -r '.token')

echo "$TOKEN_1"
echo "$TOKEN_2"

curl -X POST localhost:8080/v1/start -H "Authorization: $TOKEN_1"

action() {
    token="$1"
    state="$2"

    declare -a cities
    readarray -t cities < <(echo "$state" | jq -r '.state.cities | keys[]')
    city1=${cities[0]}
    city2=${cities[1]}

    curl -X POST localhost:8080/v1/action -H "Authorization: $token" -d @- <<EOF
    {
        "action": {
            "attack": {
                "from": "$city1",
                "to": "$city2",
                "troops": {
                    "a": 10000
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

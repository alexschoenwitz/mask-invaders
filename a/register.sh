#!/bin/bash

curl -sX POST localhost:8080/v1/register -d "{\"name\": \"$(uuidgen)\"}"

#!/bin/sh
echo "1- $1"
echo "2- $2"
pwd
ls -l
/verify-tasks -dir "$1" -timeout "$2" | /result-md

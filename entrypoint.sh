#!/bin/sh

/verify-tasks -dir "$1" -timeout "$2" | /result-md

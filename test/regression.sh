#!/bin/bash

if [ "$1" == "-f" ]; then
    input=fastinput.json
elif [ "$1" == "-s"]; then
    input=slowinput.json
else
    echo "usage -f fast -s slow"
    exit
fi

mv output.json output-old.json
sudo -E  go install ../
Dviz -ll=5 -file=$input
cmp --silent output.json output-old.json || echo "files are different"


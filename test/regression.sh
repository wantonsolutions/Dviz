#!/bin/bash

if [ "$1" == "-f" ]
then
    input=fastinput.json
elif [ "$1" == "-s" ]
then
    input=slowinput.json
else
    echo "usage -f fast -s slow"
    exit
fi

mv output.json output-old.json
go build ../

DATE="$(date)"
echo "$1, ${DATE}" >> time.out
#server
#Dviz -ll=5 -s &
#go run ../client/client.go $input
#killall Dviz

#local
#(/usr/bin/time -f'%E' Dviz -ll=0 -file=$input) &>> time.out
/usr/bin/time -f'%E' Dviz -ll=7 -file=$input
#Dviz -ll=0 -file=$input -cpuprofile cpu.prof
cmp --silent output.json output-old.json || echo "files are different"
#tail time.out


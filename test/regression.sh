#!/bin/bash

fast="-fast"

if [ "$1" == "-f" ]
then
    input=fastinput.json
elif [ "$1" == "-s" ]
then
    input=slowinput.json
elif [ "$1" == "-b" ]
then
    input=big.json
else
    echo "usage -f fast -s slow"
    exit
fi

mv output.json output-old.json
rm Dviz
go build ../

DATE="$(date)"
echo "$1, ${DATE}" >> time.out
#server
#Dviz -ll=5 -s &
#go run ../client/client.go $input
#killall Dviz

#local
#(/usr/bin/time -f'%E' Dviz -ll=0 -file=$input) &>> time.out
#for (( i=1; i<1000 ; i+=100 ))
#do
    #echo $i
    #/usr/bin/time -f'%E' ./Dviz $fast  -ll=7 -d -itt=$i -file=$input
    /usr/bin/time -f'%E' ./Dviz $fast  -ll=7 -d -file=$input
    #./Dviz -ll=0 -file=$input $fast -cpuprofile cpu.prof -memprofile mem.prof
    #./Dviz -ll=0 -file=$input $fast -memprofile mem.prof
    cmp --silent output.json output-old.json || echo "files are different"
    evince default.pdf
#done

#tail time.out


#!/bin/bash

platform=`uname`

if [[ "$platform" == 'Linux' ]]; then
    binary_name='packetbeat_raclette_linux64'
elif [[ "$platform" == 'Darwin' ]]; then
    binary_name='packetbeat_raclette_osx64'
else
    echo "Platform $platform not supported"
fi

if ! [ -e ./$binary_name ]; then
    wget https://dd-pastebin.s3.amazonaws.com/leo/raclette/$binary_name
    chmod +x $binary_name
fi

sudo ./$binary_name -c packetbeat.yml

#!/usr/bin/env bash

distribute(){
    for line in `cat clients2.txt`
    do
       ssh "duanh"@$line "mkdir bphalanx"
       echo "---- upload client: duanh@$line\n ----"
       scp client ips.txt config.json runClient.sh closeClient.sh "duanh"@$line:~/bphalanx
       ssh "duanh"@$line "chmod 777 ~/bphalanx/runClient.sh"
       ssh "duanh"@$line "chmod 777 ~/bphalanx/closeClient.sh"
       ssh "duanh"@$line "chmod 777 ~/bphalanx/client"
    done
}

# distribute files
distribute

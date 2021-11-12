#!/usr/bin/env bash

distribute(){
    for line in `cat clients2.txt`
    do
       ssh "duanh"@$line "mkdir bphalanx"
       echo "---- upload client: duanh@$line\n ----"
       scp ips.txt config.json runClient.sh "duanh"@$line:~/bphalanx
       ssh "duanh"@$line "chmod 777 ~/bphalanx/runClient.sh"
       ssh "duanh"@$line "chmod 777 ~/bphalanx/closeClient.sh"
    done
}

# distribute files
distribute

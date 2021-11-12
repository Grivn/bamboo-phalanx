#!/usr/bin/env bash

kill_all_servers(){
    for line in `cat public_ips.txt`
    do
       ssh -t "root"@$line "rm -rf ~/bphalanx"
    done
}

# distribute files
kill_all_servers

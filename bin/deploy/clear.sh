#!/usr/bin/env bash

kill_all_servers(){
    for line in `cat public_ips.txt`
    do
       ssh -t "wanggr"@$line "rm -rf ~/bphalanx"
    done
    for line in `cat public_ips_2.txt`
    do
       ssh -t "liumm"@$line "rm -rf ~/bphalanx"
    done
}

# distribute files
kill_all_servers

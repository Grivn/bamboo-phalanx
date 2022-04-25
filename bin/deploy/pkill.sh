#!/usr/bin/env bash

kill_all_servers(){
    for line in `cat real_server.txt`
    do
       ssh -t "root"@$line "pkill server ; rm ~/bphalanx/server.pid"
    done
}

# distribute files
kill_all_servers

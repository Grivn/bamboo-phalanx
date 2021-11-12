#!/usr/bin/env bash

distribute(){
    for line in `cat public_ips.txt`
    do
      ssh "root"@$line "mkdir ~/bphalanx"
      echo "---- upload replica: root@$line \n ----"
      scp server ips.txt run.sh "root"@$line:~/bphalanx
    done
}

# distribute files
distribute

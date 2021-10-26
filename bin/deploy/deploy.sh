#!/usr/bin/env bash
sh ./pkill.sh

distribute(){
    for line in `cat public_ips.txt`
    do 
      ssh "wanggr"@$line "mkdir ~/bphalanx"
      echo "---- upload replica: wanggr@$line \n ----"
      scp server ips.txt run.sh "wanggr"@$line:~/bphalanx
    done
}

# distribute files
distribute

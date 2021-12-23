#!/usr/bin/env bash
sh ./pkill.sh

distribute(){
    for line in `cat real_server.txt`
    do
      ssh "wanggr"@$line "mkdir ~/bphalanx"
      echo "---- upload replica: wanggr@$line \n ----"
      scp server ips.txt run.sh "wanggr"@$line:~/bphalanx
    done
    for line in `cat real_server_2.txt`
    do
      ssh "liumm"@$line "mkdir ~/bphalanx"
      echo "---- upload replica: liumm@$line \n ----"
      scp server ips.txt run.sh "liumm"@$line:~/bphalanx
    done
}

# distribute files
distribute

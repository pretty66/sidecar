rm -f /sidecar/offline.lock
for (( i = 0; i < 15; i++ )); do
    res=`netstat -l | grep 38081`
    if [ -z "$res" ];then
        sleep 1
    else
      echo "sidecar start success。"
      exit
    fi
done
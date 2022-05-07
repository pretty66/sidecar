scid=`ps -A | grep ServiceCar | awk '{print $1}'`
kill -s SIGUSR1 ${scid}
echo 1 > /sidecar/offline.lock
for (( i = 0; i < 5; i++ )); do
    res=`netstat -nl | grep -E ":(80|9000|10000) "`
    if [ -z "$res" ];then
        kill ${scid}
        exit
    else
      sleep 1
    fi
done

kill ${scid}

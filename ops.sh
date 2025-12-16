#!/bin/bash

PATH=/usr/local/sbin:/sbin:/bin:/usr/sbin:/usr/bin:/root/bin

BASE="$(cd $(dirname $0); pwd)"
BIN="queryCollector"
CMD="${BASE}/${BIN} -config=${BASE}/config/config.toml -loglevel=debug"

PROC="${BIN} -config"

p_error(){
        echo "[$(date +%F_%T)] - ERROR - $*"
        exit 1
}
p_echo(){
        echo "[$(date +%F_%T)] - INFO - $*"
}

check(){
        chmod +x ${BASE}/${BIN}
        pid=$(ps aux |egrep -v "grep|$(basename $0)"  |grep "${PROC}" |awk '{print $2}')
        test -z "${pid}" && return 1 || return 0
}


start(){
        check
        test $? -eq 0 && p_error "${PROC}已运行,pid:${pid},无需启动"
        # start cmd
        nohup $CMD >> nohup.out 2>&1 &
        for ((i=0;i<5;i++)) ; do
                sleep 3
                check
                test $? -eq 0 && {
                        p_echo "${PROC} 启动成功,pid:${pid}"
                        break
                }
        done
        if [ $i -ge 5 ] ; then
                p_error "${PROC} 启动失败,请检查"
        fi
}

stop(){
        check
        test $? -eq 0 && {
                kill $pid
                sleep 3
                check
                test $? -eq 0 || p_echo "${PROC} 停止成功"
        } || {
                p_error "${PROC} 未启动"
        }
}

status(){
        check
        test $? -eq 0 && p_echo "${PROC} 运行中... pid:${pid}" || p_error "${PROC} 未运行"
}

restart(){
        stop
        sleep 1
        start
}

test $# -eq 1 && {
        $*
} || echo "Usage. sh $(basename $0) start|stop|status|restart"
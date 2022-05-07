FROM golang:1.17.8 AS builder

WORKDIR /build
COPY . .
RUN make

FROM ubuntu:latest

RUN sed -i s@/archive.ubuntu.com/@/mirrors.aliyun.com/@g /etc/apt/sources.list

RUN apt-get update -y && \
    apt-get install net-tools iproute2 -y

WORKDIR /root

COPY --from=builder /build/bin/ServiceCar /root/ServiceCar
COPY --from=builder /build/config/sc_config.yaml /root/config/sc_config.yaml

## k8s lifecycle setting
#lifecycle:
#  postStart:
#    exec:
#      command:
#        - "/bin/bash"
#        - "-c"
#        - "/root/wait-until-ready.sh"
#  preStop:
#    exec:
#      command:
#        - "/bin/bash"
#        - "-c"
#        - "/root/wait-until-stop.sh"
COPY --from=builder /build/wait-until-ready.sh /root/wait-until-ready.sh
COPY --from=builder /build/wait-until-stop.sh /root/wait-until-stop.sh

CMD /root/ServiceCar

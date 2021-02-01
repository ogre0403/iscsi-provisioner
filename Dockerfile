FROM golang:1.15.2 as build
#FROM centos:7 as build

#RUN yum update -y ; \
#yum install -y epel-release ; \
#yum install -y scsi-target-utils device-mapper-devel lvm2-devel
#
#RUN yum install -y golang gcc automake autoconf libtool make

RUN mkdir /iscsi-provisioner
WORKDIR /iscsi-provisioner

COPY . .
RUN if [ ! -d "/iscsi-provisioner/vendor" ]; then  go mod vendor; fi

RUN make build-in-docker



FROM alpine:3.7
#FROM centos:7

#RUN yum update -y ; \
#yum install -y epel-release ; \
#yum install -y scsi-target-utils device-mapper-devel lvm2-devel
#RUN yum clean all

COPY --from=build /iscsi-provisioner/bin/iscsi-provisioner /
CMD ["/iscsi-provisioner","start","-v","2", "logtostderr","true"]
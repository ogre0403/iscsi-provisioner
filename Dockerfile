FROM golang:1.15.2 as build

RUN mkdir /iscsi-provisioner
WORKDIR /iscsi-provisioner

COPY . .
RUN if [ ! -d "/iscsi-provisioner/vendor" ]; then  go mod vendor; fi

RUN make build-in-docker



FROM alpine:3.7
COPY --from=build /iscsi-provisioner/bin/iscsi-provisioner /
CMD ["/iscsi-controller","-v","2", "logtostderr","true"]
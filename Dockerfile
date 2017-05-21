FROM golang
ADD . /go/src/github.com/tobyjsullivan/event-store
RUN  go install github.com/tobyjsullivan/event-store
CMD /go/bin/event-store

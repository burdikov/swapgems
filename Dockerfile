FROM golang:latest

COPY go.mod .
COPY go.sum .
COPY main.go .
RUN go build -o app
RUN mv app /usr/bin/

EXPOSE 8080
ENTRYPOINT ["app"]
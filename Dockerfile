FROM golang:1.19

WORKDIR /src
COPY . .

RUN apt -y update;apt -y install procps psmisc telnet iputils-ping nano curl wget

RUN go mod download
RUN go build -o otero ./...

EXPOSE 8081 8082

CMD ["/otero"]

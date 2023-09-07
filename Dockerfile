FROM golang:1.21

LABEL repo="github.com/komuw/otero"

WORKDIR /src

# RUN apt -y update;apt -y install procps psmisc telnet iputils-ping nano curl wget

COPY go.mod .
RUN go mod download

COPY . .
RUN go build -o otero .

EXPOSE 8081 8082

CMD ["./otero"]

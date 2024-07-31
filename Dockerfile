FROM golang:1.22.4-alpine

WORKDIR /travel

COPY go.mod go.sum ./

RUN go mod download && go mod verify

COPY . .

WORKDIR /travel/cmd/travel

RUN go build -o /travel/bin/travel .

EXPOSE 8080
ENTRYPOINT [ "travel/bin/main" ]


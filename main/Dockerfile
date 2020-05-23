FROM golang:1.14 AS build

WORKDIR /go/src/gitub.com/gofiber/fiber

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o app ./main


FROM alpine:latest

WORKDIR /app

COPY --from=build /go/src/gitub.com/gofiber/fiber/app .

EXPOSE 80

CMD ["./app"]
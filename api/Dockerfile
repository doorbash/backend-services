FROM golang:1.17.1-alpine3.13 as builder
RUN apk --no-cache add ca-certificates
WORKDIR /go/src/app
COPY . .
RUN CGO_ENABLED=0 go build -o /app

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app /app
EXPOSE 8080
CMD [ "/app" ]
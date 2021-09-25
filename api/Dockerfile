FROM golang:1.17.1 as builder
WORKDIR /go/src/app
COPY . .
RUN CGO_ENABLED=0 go build -o /api

FROM scratch
COPY --from=builder /api /api
EXPOSE 8080
CMD [ "/api" ]
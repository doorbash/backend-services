FROM golang:1.17.1 as builder
WORKDIR /go/src/app
COPY . .
RUN CGO_ENABLED=0 go build -o /notification

FROM scratch
COPY --from=builder /notification /notification
EXPOSE 8080
CMD [ "/notification" ]
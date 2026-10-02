FROM golang:1.23-alpine AS build
WORKDIR /src
COPY services/worker-go/ ./
RUN go build -o /out/worker .

FROM alpine:3.20
COPY --from=build /out/worker /worker
ENTRYPOINT ["/worker"]

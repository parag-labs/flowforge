# Multi-stage build for the FlowForge engine.
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY services/engine-go/ ./
RUN go build -o /out/engine ./cmd/engine

FROM alpine:3.20
COPY --from=build /out/engine /engine
ENV FLOWFORGE_ADDR=:8080
EXPOSE 8080
ENTRYPOINT ["/engine"]

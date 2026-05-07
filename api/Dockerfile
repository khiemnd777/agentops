FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN go build -o /out/agentops ./cmd/server
RUN go build -o /out/agentops-cli ./cmd/agentops

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/agentops /app/agentops
COPY --from=build /out/agentops-cli /app/agentops-cli
COPY migrations /app/migrations
EXPOSE 8080
CMD ["/app/agentops"]

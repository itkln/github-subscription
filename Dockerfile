FROM golang:1.25.1-alpine AS build

WORKDIR /src

COPY . .

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/github-subscription ./cmd

FROM alpine:3.22

WORKDIR /app

COPY --from=build /out/github-subscription /app/github-subscription
COPY --from=build /src/migration /app/migration

EXPOSE 8080

CMD ["/app/github-subscription"]

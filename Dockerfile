FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/wodl ./cmd/wodl

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /bin/wodl /bin/wodl
EXPOSE 8080
ENV DB_PATH=/data/wodl.db
ENTRYPOINT ["/bin/wodl"]
